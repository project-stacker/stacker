// This package is a small go "library" (read: exec wrapper) around the
// mksquashfs binary that provides some useful primitives.
package squashfs

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
	"stackerbuild.io/stacker/pkg/log"
)

var checkZstdSupported sync.Once
var zstdIsSuspported bool

var tryKernelMountSquash bool = true
var kernelSquashMountFailed error = errors.New("kernel squash mount failed")

// ExcludePaths represents a list of paths to exclude in a squashfs listing.
// Users should do something like filepath.Walk() over the whole filesystem,
// calling AddExclude() or AddInclude() based on whether they want to include
// or exclude a particular file. Note that if e.g. /usr is excluded, then
// everyting underneath is also implicitly excluded. The
// AddExclude()/AddInclude() methods do the math to figure out what is the
// correct set of things to exclude or include based on what paths have been
// previously included or excluded.
type ExcludePaths struct {
	exclude map[string]bool
	include []string
}

func NewExcludePaths() *ExcludePaths {
	return &ExcludePaths{
		exclude: map[string]bool{},
		include: []string{},
	}
}

func (eps *ExcludePaths) AddExclude(p string) {
	for _, inc := range eps.include {
		// If /usr/bin/ls has changed but /usr hasn't, we don't want to list
		// /usr in the include paths any more, so let's be sure to only
		// add things which aren't prefixes.
		if strings.HasPrefix(inc, p) {
			return
		}
	}
	eps.exclude[p] = true
}

func (eps *ExcludePaths) AddInclude(orig string, isDir bool) {
	// First, remove this thing and all its parents from exclude.
	p := orig

	// normalize to the first dir
	if !isDir {
		p = path.Dir(p)
	}
	for {
		// our paths are all absolute, so this is a base case
		if p == "/" {
			break
		}

		delete(eps.exclude, p)
		p = filepath.Dir(p)
	}

	// now add it to the list of includes, so we don't accidentally re-add
	// anything above.
	eps.include = append(eps.include, orig)
}

func (eps *ExcludePaths) String() (string, error) {
	var buf bytes.Buffer
	for p := range eps.exclude {
		_, err := buf.WriteString(p)
		if err != nil {
			return "", err
		}
		_, err = buf.WriteString("\n")
		if err != nil {
			return "", err
		}
	}

	_, err := buf.WriteString("\n")
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func MakeSquashfs(tempdir string, rootfs string, eps *ExcludePaths, verity VerityMetadata) (io.ReadCloser, string, string, error) {
	var excludesFile string
	var err error
	var toExclude string
	var rootHash string

	if eps != nil {
		toExclude, err = eps.String()
		if err != nil {
			return nil, "", rootHash, errors.Wrapf(err, "couldn't create exclude path list")
		}
	}

	if len(toExclude) != 0 {
		excludes, err := os.CreateTemp(tempdir, "stacker-squashfs-exclude-")
		if err != nil {
			return nil, "", rootHash, err
		}
		defer os.Remove(excludes.Name())

		excludesFile = excludes.Name()
		_, err = excludes.WriteString(toExclude)
		excludes.Close()
		if err != nil {
			return nil, "", rootHash, err
		}
	}

	tmpSquashfs, err := os.CreateTemp(tempdir, "stacker-squashfs-img-")
	if err != nil {
		return nil, "", rootHash, err
	}
	tmpSquashfs.Close()
	os.Remove(tmpSquashfs.Name())
	defer os.Remove(tmpSquashfs.Name())
	args := []string{rootfs, tmpSquashfs.Name()}
	compression := GzipCompression
	if mksquashfsSupportsZstd() {
		args = append(args, "-comp", "zstd")
		compression = ZstdCompression
	}
	if len(toExclude) != 0 {
		args = append(args, "-ef", excludesFile)
	}
	cmd := exec.Command("mksquashfs", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Run(); err != nil {
		return nil, "", rootHash, errors.Wrap(err, "couldn't build squashfs")
	}

	if verity {
		rootHash, err = appendVerityData(tmpSquashfs.Name())
		if err != nil {
			return nil, "", rootHash, err
		}
	}

	blob, err := os.Open(tmpSquashfs.Name())
	if err != nil {
		return nil, "", rootHash, errors.WithStack(err)
	}

	return blob, GenerateSquashfsMediaType(compression, verity), rootHash, nil
}

// maybeKernelSquashMount - try to mount squashfile with kernel mount
//
//	if global tryKernelMountSquash is false, do not try
//	if environment variable STACKER_ALLOW_SQUASHFS_KERNEL_MOUNTS is "false", do not try.
//	try.  If it fails, log message and set tryKernelMountSquash=false.
func maybeKernelSquashMount(squashFile, extractDir string) (bool, error) {
	if !tryKernelMountSquash {
		return false, nil
	}

	const strTrue, strFalse = "true", "false"
	const envName = "STACKER_ALLOW_SQUASHFS_KERNEL_MOUNTS"
	envVal := os.Getenv(envName)
	if envVal == strFalse {
		log.Debugf("Not trying kernel mounts per %s=%s", envName, envVal)
		tryKernelMountSquash = false
		return false, nil
	} else if envVal != strTrue && envVal != "" {
		return false, errors.Errorf("%s must be '%s' or '%s', found '%s'", envName, strTrue, strFalse, envVal)
	}

	ecmd := []string{"mount", "-tsquashfs", "-oloop,ro", squashFile, extractDir}
	var output bytes.Buffer
	cmd := exec.Command(ecmd[0], ecmd[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = &output
	cmd.Stderr = cmd.Stdout
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	exitError, ok := err.(*exec.ExitError)
	if !ok {
		tryKernelMountSquash = false
		return false, errors.Errorf("Unexpected error (no-rc), in exec (%v): %v", ecmd, err)
	}

	status, ok := exitError.Sys().(syscall.WaitStatus)
	if !ok {
		tryKernelMountSquash = false
		return false, errors.Errorf("Unexpected error (no-status) in exec (%v): %v", ecmd, err)
	}

	// we can't really tell why the mount failed. mount(8) does not give a lot specific rc exits.
	log.Debugf("maybeKernelSquashMount(%s) exited %d: %s", squashFile, status.ExitStatus(), strings.TrimRight(output.String(), "\n"))
	return false, kernelSquashMountFailed
}

func findSquashfusePath() string {
	if p := which("squashfuse_ll"); p != "" {
		return p
	}
	return which("squashfuse")
}

var squashNotFound = errors.Errorf("squashfuse program not found")

// squashFuse - mount squashFile to extractDir
// return a pointer to the squashfuse cmd.
// The caller of the this is responsible for the process created.
func squashFuse(squashFile, extractDir string) (*exec.Cmd, error) {
	sqfuse := findSquashfusePath()
	var cmd *exec.Cmd
	if sqfuse == "" {
		return cmd, squashNotFound
	}

	// given extractDir of path/to/some/dir[/], log to path/to/some/.dir-squashfs.log
	extractDir = strings.TrimSuffix(extractDir, "/")

	var cmdOut io.Writer
	var err error

	logf := filepath.Join(path.Dir(extractDir), "."+filepath.Base(extractDir)+"-squashfuse.log")
	if cmdOut, err = os.OpenFile(logf, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644); err != nil {
		log.Infof("Failed to open %s for write: %v", logf, err)
		return cmd, err
	}

	fiPre, err := os.Lstat(extractDir)
	if err != nil {
		return cmd, errors.Wrapf(err, "Failed stat'ing %q", extractDir)
	}
	if fiPre.Mode()&os.ModeSymlink != 0 {
		return cmd, errors.Errorf("Refusing to mount onto a symbolic linkd")
	}

	// It would be nice to only enable debug (or maybe to only log to file at all)
	// if 'stacker --debug', but we do not have access to that info here.
	// to debug squashfuse, use "allow_other,debug"
	cmd = exec.Command(sqfuse, "-f", "-o", "allow_other,debug", squashFile, extractDir)
	cmd.Stdin = nil
	cmd.Stdout = cmdOut
	cmd.Stderr = cmdOut
	cmdOut.Write([]byte(fmt.Sprintf("# %s\n", strings.Join(cmd.Args, " "))))
	log.Debugf("Extracting %s -> %s with %s [%s]", squashFile, extractDir, sqfuse, logf)
	err = cmd.Start()
	if err != nil {
		return cmd, err
	}

	// now poll/wait for one of 3 things to happen
	// a. child process exits - if it did, then some error has occurred.
	// b. the directory Entry is different than it was before the call
	//    to sqfuse.  We have to do this because we do not have another
	//    way to know when the mount has been populated.
	//    https://github.com/vasi/squashfuse/issues/49
	// c. a timeout (timeLimit) was hit
	startTime := time.Now()
	timeLimit := 30 * time.Second
	go func() {
		cmd.Wait()
	}()
	for count := 0; !fileChanged(fiPre, extractDir); count++ {
		if cmd.ProcessState != nil {
			// process exited, the Wait() call in the goroutine above
			// caused ProcessState to be populated.
			return cmd, errors.Errorf("squashFuse mount of %s with %s exited unexpectedly with %d", squashFile, sqfuse, cmd.ProcessState.ExitCode())
		}
		if time.Since(startTime) > timeLimit {
			cmd.Process.Kill()
			return cmd, errors.Wrapf(err, "Gave up on squashFuse mount of %s with %s after %s", squashFile, sqfuse, timeLimit)
		}
		if count%10 == 1 {
			log.Debugf("%s is not yet mounted...(%s)", extractDir, time.Since(startTime))
		}
		time.Sleep(time.Duration(50 * time.Millisecond))
	}

	return cmd, nil
}

func ExtractSingleSquash(squashFile string, extractDir string, storageType string) error {
	err := os.MkdirAll(extractDir, 0755)
	if err != nil {
		return err
	}

	if mounted, err := maybeKernelSquashMount(squashFile, extractDir); err == nil && mounted {
		return nil
	} else if err != kernelSquashMountFailed {
		return err
	}

	cmd, err := squashFuse(squashFile, extractDir)
	if err == nil {
		if err := cmd.Process.Release(); err != nil {
			return errors.Errorf("Failed to release process %s: %v", cmd, err)
		}
		return nil
	} else if err != squashNotFound {
		return err
	}
	if p := which("unsquashfs"); p != "" {
		log.Debugf("Extracting %s -> %s with unsquashfs -f -d %s %s", extractDir, squashFile, extractDir, squashFile)
		cmd := exec.Command("unsquashfs", "-f", "-d", extractDir, squashFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = nil
		return cmd.Run()
	}
	return errors.Errorf("Unable to extract squash archive %s", squashFile)
}

func mksquashfsSupportsZstd() bool {
	checkZstdSupported.Do(func() {
		var stdoutBuffer strings.Builder
		var stderrBuffer strings.Builder

		cmd := exec.Command("mksquashfs", "--help")
		cmd.Stdout = &stdoutBuffer
		cmd.Stderr = &stderrBuffer

		// Ignore errs here as `mksquashfs --help` exit status code is 1
		_ = cmd.Run()

		if strings.Contains(stdoutBuffer.String(), "zstd") ||
			strings.Contains(stderrBuffer.String(), "zstd") {
			zstdIsSuspported = true
		}
	})

	return zstdIsSuspported
}

// which - like the unix utility, return empty string for not-found.
// this might fit well in lib/, but currently lib's test imports
// squashfs creating a import loop.
func which(name string) string {
	return whichSearch(name, strings.Split(os.Getenv("PATH"), ":"))
}

func whichSearch(name string, paths []string) string {
	var search []string

	if strings.ContainsRune(name, os.PathSeparator) {
		if filepath.IsAbs(name) {
			search = []string{name}
		} else {
			search = []string{"./" + name}
		}
	} else {
		search = []string{}
		for _, p := range paths {
			search = append(search, filepath.Join(p, name))
		}
	}

	for _, fPath := range search {
		if err := unix.Access(fPath, unix.X_OK); err == nil {
			return fPath
		}
	}

	return ""
}

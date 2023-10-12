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
	"stackerbuild.io/stacker/pkg/mount"
)

var checkZstdSupported sync.Once
var zstdIsSuspported bool

var exPolInfo struct {
	once   sync.Once
	err    error
	policy *ExtractPolicy
}

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

func isMountedAtDir(src, dest string) (bool, error) {
	dstat, err := os.Stat(dest)
	if os.IsNotExist(err) {
		return false, nil
	}
	if !dstat.IsDir() {
		return false, nil
	}
	mounts, err := mount.ParseMounts("/proc/self/mountinfo")
	if err != nil {
		return false, err
	}

	fdest, err := filepath.Abs(dest)
	if err != nil {
		return false, err
	}
	for _, m := range mounts {
		if m.Target == fdest {
			return true, nil
		}
	}

	return false, nil
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

type ExtractPolicy struct {
	Extractors  []SquashExtractor
	Extractor   SquashExtractor
	Excuses     map[string]error
	initialized bool
	mutex       sync.Mutex
}

type SquashExtractor interface {
	Name() string
	IsAvailable() error
	// Mount - Mount or extract path to dest.
	//   Return nil on "already extracted"
	//   Return error on failure.
	Mount(path, dest string) error
}

func NewExtractPolicy(args ...string) (*ExtractPolicy, error) {
	p := &ExtractPolicy{
		Extractors: []SquashExtractor{},
		Excuses:    map[string]error{},
	}

	allEx := []SquashExtractor{
		&KernelExtractor{},
		&SquashFuseExtractor{},
		&UnsquashfsExtractor{},
	}
	byName := map[string]SquashExtractor{}
	for _, i := range allEx {
		byName[i.Name()] = i
	}

	for _, i := range args {
		extractor, ok := byName[i]
		if !ok {
			return nil, errors.Errorf("Unknown extractor: '%s'", i)
		}
		excuse := extractor.IsAvailable()
		if excuse != nil {
			p.Excuses[i] = excuse
			continue
		}
		p.Extractors = append(p.Extractors, extractor)
	}
	return p, nil
}

type UnsquashfsExtractor struct {
	mutex sync.Mutex
}

func (k *UnsquashfsExtractor) Name() string {
	return "unsquashfs"
}

func (k *UnsquashfsExtractor) IsAvailable() error {
	if which("unsquashfs") == "" {
		return errors.Errorf("no 'unsquashfs' in PATH")
	}
	return nil
}

func (k *UnsquashfsExtractor) Mount(squashFile, extractDir string) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	// check if already extracted
	empty, err := isEmptyDir(extractDir)
	if err != nil {
		return errors.Wrapf(err, "Error checking for empty dir")
	}
	if !empty {
		return nil
	}

	log.Debugf("unsquashfs %s -> %s", squashFile, extractDir)
	cmd := exec.Command("unsquashfs", "-f", "-d", extractDir, squashFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	err = cmd.Run()

	// on failure, remove the directory
	if err != nil {
		if rmErr := os.RemoveAll(extractDir); rmErr != nil {
			log.Errorf("Failed to remove %s after failed extraction of %s: %v", extractDir, squashFile, rmErr)
		}
		return err
	}

	// assert that extraction must create files. This way we can assume non-empty dir above
	// was populated by unsquashfs.
	empty, err = isEmptyDir(extractDir)
	if err != nil {
		return errors.Errorf("Failed to read %s after successful extraction of %s: %v",
			extractDir, squashFile, err)
	}
	if empty {
		return errors.Errorf("%s was an empty fs image", squashFile)
	}

	return nil
}

type KernelExtractor struct {
	mutex sync.Mutex
}

func (k *KernelExtractor) Name() string {
	return "kmount"
}

func (k *KernelExtractor) IsAvailable() error {
	if !amHostRoot() {
		return errors.Errorf("not host root")
	}
	return nil
}

func (k *KernelExtractor) Mount(squashFile, extractDir string) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if mounted, err := isMountedAtDir(squashFile, extractDir); err != nil {
		return err
	} else if mounted {
		return nil
	}

	ecmd := []string{"mount", "-tsquashfs", "-oloop,ro", squashFile, extractDir}
	var output bytes.Buffer
	cmd := exec.Command(ecmd[0], ecmd[1:]...)
	cmd.Stdin = nil
	cmd.Stdout = &output
	cmd.Stderr = cmd.Stdout
	err := cmd.Run()
	if err == nil {
		return nil
	}

	var retErr error

	exitError, ok := err.(*exec.ExitError)
	if !ok {
		retErr = errors.Errorf("kmount(%s) had unexpected error (no-rc), in exec (%v): %v",
			squashFile, ecmd, err)
	} else if status, ok := exitError.Sys().(syscall.WaitStatus); !ok {
		retErr = errors.Errorf("kmount(%s) had unexpected error (no-status), in exec (%v): %v",
			squashFile, ecmd, err)
	} else {
		retErr = errors.Errorf("kmount(%s) exited %d: %v", squashFile, status.ExitStatus(), output.String())
	}

	return retErr
}

type SquashFuseExtractor struct {
	mutex sync.Mutex
}

func (k *SquashFuseExtractor) Name() string {
	return "squashfuse"
}

func (k *SquashFuseExtractor) IsAvailable() error {
	if findSquashfusePath() == "" {
		return errors.Errorf("no 'squashfuse' in PATH")
	}
	return nil
}

func (k *SquashFuseExtractor) Mount(squashFile, extractDir string) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if mounted, err := isMountedAtDir(squashFile, extractDir); mounted && err == nil {
		log.Debugf("[%s] %s already mounted -> %s", k.Name(), squashFile, extractDir)
		return nil
	} else if err != nil {
		return err
	}

	cmd, err := squashFuse(squashFile, extractDir)
	if err != nil {
		return err
	}

	log.Debugf("squashFuse mounted (%d) %s -> %s", cmd.Process.Pid, squashFile, extractDir)
	if err := cmd.Process.Release(); err != nil {
		return errors.Errorf("Failed to release process %s: %v", cmd, err)
	}
	return nil
}

// ExtractSingleSquashPolicy - extract squashfile to extractDir
func ExtractSingleSquashPolicy(squashFile, extractDir string, policy *ExtractPolicy) error {
	const initName = "init"
	if policy == nil {
		return errors.Errorf("policy cannot be nil")
	}

	// avoid taking a lock if already initialized (possibly premature optimization)
	if !policy.initialized {
		policy.mutex.Lock()
		// We may have been waiting on the initializer. If so, then the policy will now be initialized.
		// if not, then we are the initializer.
		if !policy.initialized {
			defer policy.mutex.Unlock()
			defer func() {
				policy.initialized = true
			}()
		} else {
			policy.mutex.Unlock()
		}
	}

	err := os.MkdirAll(extractDir, 0755)
	if err != nil {
		return err
	}

	fdest, err := filepath.Abs(extractDir)
	if err != nil {
		return err
	}

	if policy.initialized {
		if err, ok := policy.Excuses[initName]; ok {
			return err
		}
		return policy.Extractor.Mount(squashFile, fdest)
	}

	// At this point we are the initialzer
	if policy.Excuses == nil {
		policy.Excuses = map[string]error{}
	}

	if policy.Extractors == nil || len(policy.Extractors) == 0 {
		policy.Excuses[initName] = errors.Errorf("policy had no extractors")
		return policy.Excuses[initName]
	}

	var extractor SquashExtractor
	allExcuses := []string{}
	for _, extractor = range policy.Extractors {
		err = extractor.Mount(squashFile, fdest)
		if err == nil {
			policy.Extractor = extractor
			log.Debugf("Selected squashfs extractor %s", extractor.Name())
			return nil
		}
		policy.Excuses[extractor.Name()] = err
	}

	for n, exc := range policy.Excuses {
		allExcuses = append(allExcuses, fmt.Sprintf("%s: %v", n, exc))
	}

	// nothing worked. populate Excuses[initName]
	policy.Excuses[initName] = errors.Errorf("No suitable extractor found:\n  " + strings.Join(allExcuses, "\n  "))
	return policy.Excuses[initName]
}

// ExtractSingleSquash - extract the squashFile to extractDir
// Initialize a extractPolicy struct and then call ExtractSingleSquashPolicy
// wik()th that.
func ExtractSingleSquash(squashFile string, extractDir string) error {
	exPolInfo.once.Do(func() {
		const envName = "STACKER_SQUASHFS_EXTRACT_POLICY"
		const defPolicy = "kmount squashfuse unsquashfs"
		val := os.Getenv(envName)
		if val == "" {
			val = defPolicy
		}
		exPolInfo.policy, exPolInfo.err = NewExtractPolicy(strings.Fields(val)...)
		if exPolInfo.err == nil {
			for k, v := range exPolInfo.policy.Excuses {
				log.Debugf(" squashfs extractor %s is not available: %v", k, v)
			}
		}
	})

	if exPolInfo.err != nil {
		return exPolInfo.err
	}

	return ExtractSingleSquashPolicy(squashFile, extractDir, exPolInfo.policy)
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

func isEmptyDir(path string) (bool, error) {
	fh, err := os.Open(path)
	if err != nil {
		return false, err
	}

	_, err = fh.ReadDir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
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

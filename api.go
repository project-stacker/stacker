package stacker

import "fmt"

const (
	GitVersionAnnotation      = "%s.stacker.git_version"
	StackerContentsAnnotation = "%s.stacker.stacker_yaml"
)

func getGitVersionAnnotation(namespace string) string {
	return fmt.Sprintf(GitVersionAnnotation, namespace)
}

func getStackerContentsAnnotation(namespace string) string {
	return fmt.Sprintf(StackerContentsAnnotation, namespace)
}

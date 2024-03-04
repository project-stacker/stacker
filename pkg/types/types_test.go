package types

import (
	"os"
	"reflect"
	"testing"
)

func parse(t *testing.T, content string) *Stackerfile {
	tf, err := os.CreateTemp("", "stacker_test_")
	if err != nil {
		t.Fatalf("couldn't create tempfile: %s", err)
	}
	defer tf.Close()
	defer os.Remove(tf.Name())

	_, err = tf.WriteString(content)
	if err != nil {
		t.Fatalf("couldn't write content: %s", err)
	}

	sf, err := NewStackerfile(tf.Name(), false, nil)
	if err != nil {
		t.Fatalf("failed to parse %s\n\n%s", content, err)
	}

	return sf
}

func TestDockerFrom(t *testing.T) {
	content := `meshuggah:
    from:
        type: docker
`
	sf := parse(t, content)
	l, ok := sf.Get("meshuggah")
	if !ok {
		t.Fatalf("missing meshuggah layer")
	}

	if l.From.Type != DockerLayer {
		t.Fatalf("bad type : %v", l.From)
	}

	if l.From.Tag != "" {
		t.Fatalf("bad tag")
	}

	if l.From.Url != "" {
		t.Fatalf("bad url")
	}
}

func TestDependencyOrder(t *testing.T) {
	content := `first:
    from:
        type: tar
        url: http://example.com/tar.gz
second:
    from:
        type: built
        tag: first
third:
    from:
        type: built
        tag: second
`
	sf := parse(t, content)
	do, err := sf.DependencyOrder(StackerFiles{})
	if err != nil {
		t.Fatalf("%s", err)
	}
	if len(do) != 3 {
		t.Fatalf("bad do: %v", do)
	}

	if do[0] != "first" || do[1] != "second" || do[2] != "third" {
		t.Fatalf("bad do: %v", do)
	}
}

func TestSubstitute(t *testing.T) {
	// test supported double bracket syntax with default values
	s := "${{ONE}} ${{TWO:}} ${{TWO:3}} ${{TWO2:22}} ${{THREE:3}} ${{NOTHING:}}"
	result, err := substitute(s, []string{"ONE=1", "TWO=2", "BLEH=BLAH"})
	if err != nil {
		t.Fatalf("failed substitutition: %s", err)
	}

	expected := "1 2 2 22 3 "
	if result != expected {
		t.Fatalf("bad substitution result, expected '%s' got '%s'", expected, result)
	}

	// Test that unsupported single bracket syntax is passed through
	// (there are warnings printed to help debug)
	_, err = substitute("${A} ${A:Z}", []string{"A=B"})
	if err == nil {
		t.Fatalf("substitution with unsupported placeholders should have failed but didn't")
	}

	// $PRODUCT or ${PRODUCT} are NOT valid placeholders, but also not errors.
	// we warn if you provide PRODUCT and we see those invalid placeholders
	// we do not warn if you have e.g. $DONTWARN but it isn't a provided sub
	s = "$DONTWARN $PRODUCT ${PRODUCT} ${PRODUCT:def} ${PRODUCT//x} ${{PRODUCT}} ${{PRODUCT:bar}}"
	_, err = substitute(s, []string{"PRODUCT=foo"})
	if err == nil {
		t.Fatalf("substitution with unsupported placeholders should have failed but didn't")
	}

	// a double bracket placeholder with no default value must be provided
	s = "${{NOT_SET}}"
	_, err = substitute(s, []string{})
	if err == nil {
		t.Fatalf("expected error for unset variable!")
	}
}

func TestFilterEnv(t *testing.T) {
	myenv := map[string]string{
		"PT_K1":  "val1",
		"PT_9":   "val2",
		"PTNAME": "foo",
		"TARGET": "build",
		"HOME":   "/home/user1",
	}
	var result, expected map[string]string
	var err error
	result, err = filterEnv([]string{"PT_.*", "TARGET"}, myenv)
	if err != nil {
		t.Fatalf("Failed filterEnv1: %s", err)
	}
	expected = map[string]string{
		"PT_K1": "val1", "PT_9": "val2", "TARGET": "build"}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Incorrect result Filter1 expected != found: %v != %v",
			expected, result)
	}
}

func TestBuildEnvEqualInEnviron(t *testing.T) {
	mockOsEnv := func() []string {
		return []string{"VAR_NORMAL=VAL", "VAR_TRICKY=VAL=EQUAL"}
	}

	result, err := buildEnv([]string{"VAR_.*"},
		map[string]string{"myvar": "myval"}, mockOsEnv)
	if err != nil {
		t.Fatalf("Failed buildEnv: %s", err)
	}
	expected := map[string]string{
		"VAR_NORMAL": "VAL",
		"VAR_TRICKY": "VAL=EQUAL",
		"myvar":      "myval",
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Incorrect result buildEnv expected != found: %v != %v",
			expected, result)
	}
}

func TestContainerProxyPassThroughByDefault(t *testing.T) {
	mockOsEnv := func() []string {
		return []string{"HTTP_PROXY=http://proxy.example.com", "HOME=/home/user"}
	}
	result, err := buildEnv([]string{}, map[string]string{"k": "v"}, mockOsEnv)
	if err != nil {
		t.Fatalf("Failed buildEnv: %s", err)
	}
	expected := map[string]string{
		"HTTP_PROXY": "http://proxy.example.com",
		"k":          "v",
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Incorrect result buildEnv expected != found: %v != %v",
			expected, result)
	}
}

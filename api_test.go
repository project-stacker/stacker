package stacker

import (
	"io/ioutil"
	"os"
	"testing"
)

func parse(t *testing.T, content string) *Stackerfile {
	tf, err := ioutil.TempFile("", "stacker_test_")
	if err != nil {
		t.Fatalf("couldn't create tempfile: %s", err)
	}
	defer tf.Close()
	defer os.Remove(tf.Name())

	_, err = tf.WriteString(content)
	if err != nil {
		t.Fatalf("couldn't write content: %s", err)
	}

	sf, err := NewStackerfile(tf.Name(), nil)
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

	if l.From.Type != DockerType {
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
	do, err := sf.DependencyOrder()
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
	s := "$ONE $TWO ${TWO} ${TWO:} ${TWO:3} ${THREE:3}"
	result, err := substitute(s, []string{"ONE=1", "TWO=2"})
	if err != nil {
		t.Fatalf("failed substitutition: %s", err)
	}

	expected := "1 2 2 2 2 3"
	if result != expected {
		t.Fatalf("bad substitution result, expected %s got %s", result, expected)
	}
}

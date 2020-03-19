package serfer_test

import (
	"testing"

	"github.com/johandry/serfer"
)

func TestTags(t *testing.T) {
	s, err := serfer.NewDefault(nil)
	if err != nil {
		t.Errorf("Expected no error creating the Serfer, but got %s", err)
	}

	tn := "role"
	tv := "leader"
	s.SetTag(tn, tv)

	tags := s.Tags()
	var actualValue string
	var ok bool
	if actualValue, ok = tags[tn]; !ok {
		t.Errorf("Expected tag %q to set, but it's not", tn)
	}
	if actualValue != tv {
		t.Errorf("Expected tag value to be %q, but got %q", tv, actualValue)
	}

	tn1 := "bar"
	tv1 := "foo"
	newTags := map[string]string{
		tn1:    tv1,
		"ping": "pong",
	}
	s.BindTags(newTags)

	_, ok = s.Tag(tn)
	if ok == true {
		t.Errorf("Expected not to find tag %q, but was found", tn)
	}

	actualValue, ok = s.Tag(tn1)
	if !ok {
		t.Errorf("Expected to find tag %q, but was not found", tn1)
	}
	if actualValue != tv1 {
		t.Errorf("Expected tag %q to be %q, but got %q", tn1, tv1, actualValue)
	}

}

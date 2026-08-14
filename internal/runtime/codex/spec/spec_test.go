package spec

import "testing"

func TestParse(t *testing.T) {
	got := Parse("gpt-5.6-sol:high")
	if got.Model != "gpt-5.6-sol" || got.Effort != "high" {
		t.Fatalf("%#v", got)
	}
	got = Parse(" gpt-5.6-terra ")
	if got.Model != "gpt-5.6-terra" || got.Effort != "" {
		t.Fatalf("%#v", got)
	}
	if !Parse("").Empty() {
		t.Fatal("empty")
	}
	if Parse("gpt-5.6-sol:high").String() != "gpt-5.6-sol:high" {
		t.Fatal(Parse("gpt-5.6-sol:high").String())
	}
	if !ValidEffort("xhigh") || ValidEffort("super") {
		t.Fatal("effort")
	}
	if !IsGlob("gpt-5.6-*:high") || IsGlob("gpt-5.6-sol:high") {
		t.Fatal("glob")
	}
	if !IsGlob("gpt-*-sol:high") || !IsGlob("gpt-*-terra:medium") {
		t.Fatal("family glob")
	}
}

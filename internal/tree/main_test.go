package tree

import (
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden tree file")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

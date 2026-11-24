package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinsMatchInstallerAndPackage(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, name := range []string{"install.sh", "packaging/arch/PKGBUILD"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, pin := range []string{EnSQLiteSHA256, ManifestSHA256} {
			if !strings.Contains(string(data), pin) {
				t.Errorf("%s does not pin %s", name, pin)
			}
		}
	}
}

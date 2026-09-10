package plugin

import (
	"strings"
	"testing"

	_ "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	_ "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// Replacing a module path directly in generated raw descriptors corrupts their
// length-prefixed fields. Check metadata without relying on -race/checkptr to
// catch the resulting unsafe scalar/pointer interpretation during serialization.
func TestGeneratedDescriptors(t *testing.T) {
	seen := map[protoreflect.FullName]bool{}
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(file.Path(), "sqld/v1/") {
			return true
		}
		seen[file.Package()] = true
		t.Run(file.Path(), func(t *testing.T) {
			if file.Syntax() != protoreflect.Proto3 {
				t.Errorf("syntax = %s, want proto3", file.Syntax())
			}
			pkg := strings.TrimPrefix(string(file.Package()), "sqld.v1.")
			want := "github.com/gopherex/sqld/pkg/proto/sqld/v1/" + pkg + ";" + pkg + "v1"
			if got := protodesc.ToFileDescriptorProto(file).GetOptions().GetGoPackage(); got != want {
				t.Errorf("go_package = %q, want %q", got, want)
			}
		})
		return true
	})
	for _, pkg := range []protoreflect.FullName{"sqld.v1.ir", "sqld.v1.plugin"} {
		if !seen[pkg] {
			t.Errorf("no descriptors registered for %s", pkg)
		}
	}
}

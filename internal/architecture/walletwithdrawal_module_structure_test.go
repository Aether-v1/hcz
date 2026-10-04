package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestWalletWithdrawalModuleStructure(t *testing.T) {
	root := findRepositoryRoot(t)
	base := filepath.Join(root, "internal", "modules", "walletwithdrawal")

	requiredFiles := []string{
		"domain/withdrawal.go",
		"domain/state_machine.go",
		"contract/types.go",
		"contract/ports.go",
		"contract/errors.go",
		"application/service.go",
		"application/create.go",
		"application/cancel.go",
		"application/admin.go",
		"application/query.go",
		"application/address.go",
		"application/fee.go",
		"application/quote.go",
		"infrastructure/gormstore/store.go",
		"transport/http/user_handler.go",
		"transport/http/admin_handler.go",
		"transport/http/address_handler.go",
		"transport/http/routes.go",
		"transport/presenter/withdrawal.go",
	}
	for _, rel := range requiredFiles {
		full := filepath.Join(base, filepath.FromSlash(rel))
		if _, err := os.Stat(full); err != nil {
			t.Errorf("expected walletwithdrawal file %s to exist", rel)
		}
	}

	assertDeclared(t, base, "domain/state_machine.go", "CanTransition")
	assertDeclared(t, base, "domain/withdrawal.go", "Withdrawal", "Address")
	assertDeclared(t, base, "application/service.go", "Service", "NewService")
	assertDeclared(t, base, "application/create.go", "CreateWithdrawal")
	assertDeclared(t, base, "application/cancel.go", "CancelWithdrawal")
	assertDeclared(t, base, "application/admin.go", "Approve", "Reject", "MarkProcessing", "Complete")
	assertDeclared(t, base, "infrastructure/gormstore/store.go", "Store", "New")
	assertDeclared(t, base, "transport/http/user_handler.go", "UserHandler", "NewUserHandler")
	assertDeclared(t, base, "transport/http/admin_handler.go", "AdminHandler", "NewAdminHandler")
	assertDeclared(t, base, "transport/http/routes.go", "RegisterUserRoutes", "RegisterAdminRoutes")
}

func assertDeclared(t *testing.T, base, relFile string, names ...string) {
	t.Helper()
	full := filepath.Join(base, filepath.FromSlash(relFile))
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, full, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse %s: %v", relFile, err)
	}
	declared := make(map[string]struct{})
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name != nil {
				declared[d.Name.Name] = struct{}{}
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					declared[s.Name.Name] = struct{}{}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						declared[n.Name] = struct{}{}
					}
				}
			}
		}
	}
	for _, name := range names {
		if _, ok := declared[name]; !ok {
			t.Errorf("%s: expected declaration %q", relFile, name)
		}
	}
}

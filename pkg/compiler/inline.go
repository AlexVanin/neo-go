package compiler

import (
	"go/ast"
	"go/types"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// inlineCall inlines call of n for function represented by f.
// Call `f(a,b)` for definition `func f(x,y int)` is translated to block:
// {
//    x := a
//    y := b
//    <inline body of f directly>
// }
func (c *codegen) inlineCall(f *funcScope, n *ast.CallExpr) {
	pkg := c.buildInfo.program.Package(f.pkg.Path())
	sig := c.typeOf(n.Fun).(*types.Signature)

	// Arguments need to be walked with the current scope,
	// while stored in the new.
	oldScope := c.scope.vars.locals
	c.scope.vars.newScope()
	newScope := c.scope.vars.locals
	defer c.scope.vars.dropScope()
	for i := range n.Args {
		c.scope.vars.locals = oldScope
		name := sig.Params().At(i).Name()
		if arg, ok := n.Args[i].(*ast.Ident); ok {
			// When function argument is variable or const, we may avoid
			// introducing additional variables for parameters.
			// This is done by providing additional alias to variable.
			if vt, index := c.scope.vars.getVarIndex(arg.Name); index != -1 {
				c.scope.vars.locals = newScope
				c.scope.vars.addAlias(name, vt, index)
				continue
			}
		}
		ast.Walk(c, n.Args[i])
		c.scope.vars.locals = newScope
		c.scope.newLocal(name)
		c.emitStoreVar("", name)
	}

	c.pkgInfoInline = append(c.pkgInfoInline, pkg)
	oldMap := c.importMap
	c.fillImportMap(f.file, pkg.Pkg)
	ast.Inspect(f.decl, c.scope.analyzeVoidCalls)
	ast.Walk(c, f.decl.Body)
	if c.scope.voidCalls[n] {
		for i := 0; i < f.decl.Type.Results.NumFields(); i++ {
			emit.Opcodes(c.prog.BinWriter, opcode.DROP)
		}
	}
	c.importMap = oldMap
	c.pkgInfoInline = c.pkgInfoInline[:len(c.pkgInfoInline)-1]
}

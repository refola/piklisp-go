// convert a Node into Go code

/* TYPES OF GO SYNTAX TO HANDLE

===== CONTEXTS =====

Top-level: Anything that's valid outside of, e.g., a function body.
* package declaration
* import
* top-level consts and vars
* functions

Action: Places that require the program to _do_ something.
* bodies of functions
* bodies of control structures

Value: Places that need something that results in a single value.
* function arguments
* right-hand-side of ":=" and friends
* most args for if/for/switch
* array indices
* several other places

SimpleStmt (see golang.org/ref/spec#SimpleStmt): Anything except for control structures.
* values, channel sends, ++/--, assignments, and short declarations
* found at beginnings of control structures


===== SYNTAXES BY CONTEXT =====

Top-level:
* (first args) → first args
** package
* (first args ...)
→ first (args; ...)
** import
* (first (arg1 ...) (arg2 ...) ...)
→ first ( arg1 ...; arg2 ...; ... )
** const
** var
* (first second (args1 ...) (args2 ...) (args3 ...))
→ first second(args1, ...) (args2, ...) { args3; ...; }
** func

Action:
* (first second args ...)
→ second first args ...
** = := /= *= += -=
* (first args ...)
→ first(args ...)
** function calls
* (first (arg1 ...) (arg2 ...) ...)
→ first arg1 { ... } ??? arg2 { ... } ...
** if/for/switch/select
* (first second)
→ first second
** return
** goto
** label?

Value:
* (first second third)
→ (second first third)
** math expressions
** comparisons
* (first args ...)
→ first(args ...)
** function calls


===== STRATEGY FOR HANDLING THESE CASES =====

* start at top level
* have each function for handling these cases track what types of things it's expecting
* use per-context function maps to choose how to process each node

*/

package parse

import "fmt"

const NODE_GOSTRING_DEBUG = true

type nodeType int

const (
	topNode nodeType = iota
	actionNode
	valueNode
)

func (t nodeType) String() string {
	return map[nodeType]string{
		topNode:    "topNode",
		actionNode: "actionNode",
		valueNode:  "valueNode",
	}[t]
}

// Process a top-level Node, starting from its first child
func nodeProcessTop(first *Node) string {
	var f func(*Node) string
	switch first.content {
	case "package":
		f = nodeUnparenTwo
	case "import":
		f = nodeImport
	case "const", "var":
		f = nodeConstVar
	case "func":
		f = nodeFunc
	default:
		// TODO: This seems related to the f(n) or f(n.first) function
		// call in the anonymous function inside nodeProcess. Maybe I
		// should make a "get first" function that returns the passed node
		// if it has non-empty content, or its first child otherwise.
		panic("Unknown top-level node type: " + first.content)
	}
	return f(first)
}

// Process an action Node, starting from its first child
func nodeProcessAction(first *Node) string {
	if NODE_GOSTRING_DEBUG {
		fmt.Println("About to try checking the action Node's type.")
	}
	var f func(*Node) string
	switch first.content {
	case "=", ":=", "+=", "-=", "*=", "/=":
		f = nodeAssign
	case "if", "for", "switch", "select":
		f = nodeControlBlock
	case "return":
		f = nodeUnparenTwo
	default:
		f = nodeFuncall
	}
	if NODE_GOSTRING_DEBUG {
		fmt.Printf("About to try applying %v to the Node.\n", f)
	}
	return f(first)
}

// Process a value Node, notably NOT starting from its first
// child. Value literals don't have that structure.
func nodeProcessValue(n *Node) string {
	// TODO: This function's requirement of not starting from the first
	// child does not match the other nodeProcessFoo functions. This
	// inconsistency needs to be fixed.

	if n.content != "" {
		return n.content
	}
	var f func(*Node) string
	switch n.content {
	case "+", "-", "*", "/", "==", "!=", ">=", "<=", "<", ">":
		f = nodeMath
	default:
		f = nodeFuncall
	}
	return f(n)
}

// Apply the correct node-parsing action for the given node and all
// its same-level successors
func nodeProcess(first *Node, t nodeType) string {
	f, okay := map[nodeType](func(*Node) string){
		topNode:    nodeProcessTop,
		actionNode: nodeProcessAction,
		valueNode:  nodeProcessValue,
	}[t]
	if !okay {
		panic(fmt.Errorf("Unknown type of node: %v", t))
	}
	out := ""
	for n := first; n != nil; n = n.next {
		result, err := func() (out string, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("Recovered panic: %v", r)
				}
			}()
			// TODO: Something's screwy here. I think I've made an unchecked
			// inconsistent interface for answering "does this function want
			// the parent Node or the first child?". Changing between n and
			// n.first changes how the program panics on trying to get Hello
			// World working again.
			out = f(n)
			return
		}()
		if err != nil {
			panic(fmt.Errorf("Could not process code as %v: %v\n Got error: %v", t, n, err))
		} else {
			out += result + "\n"
		}
	}
	return out
}

// Convert a Node into Go code.
func (n *Node) GoString() string { return nodeProcess(n.first, topNode) }

// Return the contents of the given Node and its next Node, separated
// by a space.
func nodeUnparenTwo(first *Node) string {
	return first.content + " " + first.next.content
}

// Convert an import Node into a Go import command.
func nodeImport(first *Node) string {
	out := "import ("
	for n := first.next; n != nil; n = n.next {
		out += n.content + "; "
	}
	out += ")"
	return out
}

// Convert a const or var Node into a top-level Go const or var
// declaration.
func nodeConstVar(first *Node) string {
	out := first.content + "("
	for n := first.next; n != nil; n = n.next {
		out += nodeContents(n.first) + "\n"
	}
	out += ")"
	return out
}

// Generate a space-separated list of node contents.
func nodeContents(first *Node) string {
	out := ""
	for n := first; n != nil; n = n.next {
		out += n.content + " "
	}
	return out
}

// Convert a function Node into a Go function declaration.
func nodeFunc(first *Node) string {
	// "func"
	n := first
	out := n.content
	// function name
	n = n.next
	out += " " + n.content
	// function args
	n = n.next
	out += "(" + nodeContents(n.first) + ")"
	// function return types
	n = n.next
	out += "(" + nodeContents(n.first) + ")"
	// function body
	n = n.next
	out += "{" + nodeProcess(n.first, actionNode) + "}"
	return out
}

// Process an assignment, starting from the first Node.
func nodeAssign(first *Node) string {
	// Go LHS and assignment operator
	out := first.next.content + first.content
	// RHS
	// TODO: Properly parse as values.
	out += nodeContents(first.next.next)
	return out
}

// Convert a function call into Go
func nodeFuncall(first *Node) string {
	if NODE_GOSTRING_DEBUG {
		fmt.Println("Converting node into a function call:")
		fmt.Println(first.content)
	}
	// TODO: I think this is where everything is breaking. It needs to
	// spit out comma-separated value parses instead of
	// newline-separated.
	return first.content + "(" + nodeProcess(first.next, valueNode) + ")"
}

// Convert if/for/switch/select statements into Go.
func nodeControlBlock(first *Node) string {
	// TODO: implement
	panic("nodeControlBlock is unimplemented!")
}

// Convert a Lisp math function call into Go form.
func nodeMath(first *Node) string {
	op := first.content
	n := first.next
	lhs := n.content
	n = n.next
	rhs := n.content
	return "(" + lhs + " " + op + " " + rhs + ")"
}

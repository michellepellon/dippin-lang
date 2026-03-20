=== Next component: formatter ===
component: formatter
package: formatter/
description: Canonical formatter: IR to .dip source

=== Current codebase ===
./cmd/dippin/main.go
./ir/edge.go
./ir/ir_test.go
./ir/ir.go
./ir/lookup.go
./ir/source.go
./validator/codes.go
./validator/diagnostic.go
./validator/lint_codes.go
./validator/lint.go
./validator/lint_test.go
./validator/validate.go
./validator/validate_test.go

=== Test status ===
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/ir
ok  	github.com/2389/dippin/validator

=== Ledger ===
component	package	status	description
ir	ir/	complete	Canonical IR types (Workflow, Node, Edge, Condition)
parser-lexer	parser/	in_progress	Line-based indentation-aware lexer
parser-core	parser/	in_progress	Parser: .dip source to IR with error recovery
parser-conditions	parser/	in_progress	Condition expression parser (and/or/not/compare)
validator	validator/	complete	Graph structure validation (DIP001-DIP009)
linter	validator/	complete	Semantic quality warnings (DIP101-DIP112)
formatter	formatter/	pending	Canonical formatter: IR to .dip source
dot-exporter	export/	pending	DOT exporter: IR to DOT string
migration	migrate/	pending	Migration tool: DOT to IR to .dip
cli	cmd/dippin/	pending	CLI subcommands: parse, validate, lint, fmt, export-dot, migrate

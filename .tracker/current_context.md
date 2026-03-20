=== Next component: migration ===
component: migration
package: migrate/
description: Migration tool: DOT to IR to .dip

=== Current codebase ===
./cmd/dippin/main.go
./export/dot_test.go
./export/dot.go
./formatter/format_test.go
./formatter/format.go
./ir/edge.go
./ir/ir_test.go
./ir/ir.go
./ir/lookup.go
./ir/source.go
./validator/codes.go
./validator/diagnostic.go
./validator/lint_codes.go
./validator/lint_test.go
./validator/lint.go
./validator/validate_test.go
./validator/validate.go

=== Test status ===
?   	github.com/2389/dippin/cmd/dippin	[no test files]
ok  	github.com/2389/dippin/export	(cached)
ok  	github.com/2389/dippin/formatter	(cached)
ok  	github.com/2389/dippin/ir	(cached)
ok  	github.com/2389/dippin/validator	(cached)

=== Ledger ===
component	package	status	description
ir	ir/	complete	Canonical IR types (Workflow, Node, Edge, Condition)
parser-lexer	parser/	complete	Line-based indentation-aware lexer
parser-core	parser/	complete	Parser: .dip source to IR with error recovery
parser-conditions	parser/	complete	Condition expression parser (and/or/not/compare)
validator	validator/	complete	Graph structure validation (DIP001-DIP009)
linter	validator/	complete	Semantic quality warnings (DIP101-DIP112)
formatter	formatter/	complete	Canonical formatter: IR to .dip source
dot-exporter	export/	complete	DOT exporter: IR to DOT string
migration	migrate/	in_progress	Migration tool: DOT to IR to .dip
cli	cmd/dippin/	pending	CLI subcommands: parse, validate, lint, fmt, export-dot, migrate

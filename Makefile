
GHERKIN_BASE ?= ..

GOOD_FEATURE_FILES = $(shell find $(GHERKIN_BASE)/testdata/good -name "*.feature")
BAD_FEATURE_FILES = $(shell find $(GHERKIN_BASE)/testdata/bad -name "*.feature")

TOKENS   = $(patsubst $(GHERKIN_BASE)/testdata/%.feature,acceptance/testdata/%.feature.tokens,$(GOOD_FEATURE_FILES))
ASTS     = $(patsubst $(GHERKIN_BASE)/testdata/%.feature,acceptance/testdata/%.feature.ast.json,$(GOOD_FEATURE_FILES))
ERRORS   = $(patsubst $(GHERKIN_BASE)/testdata/%.feature,acceptance/testdata/%.feature.errors,$(BAD_FEATURE_FILES))

GO_SOURCE_FILES = $(shell find . -name "*.go") parser.go dialects_builtin.go

export GOPATH = $(realpath ./)

all: .compared

test: $(TOKENS) $(ASTS) $(ERRORS)
.PHONY: test

.compared: .built $(TOKENS) $(ASTS) $(ERRORS)
	touch $@

.built: $(GO_SOURCE_FILES) bin/gherkin-generate-tokens bin/gherkin-generate-ast LICENSE
	touch $@

bin/gherkin-generate-tokens: $(GO_SOURCE_FILES)
	go build -o $@ ./gherkin-generate-tokens

bin/gherkin-generate-ast: $(GO_SOURCE_FILES)
	go build -o $@ ./gherkin-generate-ast

acceptance/testdata/%.feature.tokens: $(GHERKIN_BASE)/testdata/%.feature $(GHERKIN_BASE)/testdata/%.feature.tokens
	mkdir -p `dirname $@`
	bin/gherkin-generate-tokens $< > $@
	diff --unified $<.tokens $@
.DELETE_ON_ERROR: acceptance/testdata/%.feature.tokens

acceptance/testdata/%.feature.ast.json: $(GHERKIN_BASE)/testdata/%.feature $(GHERKIN_BASE)/testdata/%.feature.ast.json
	mkdir -p `dirname $@`
	bin/gherkin-generate-ast $< | jq --sort-keys "." > $@
	diff --unified $<.ast.json $@
.DELETE_ON_ERROR: acceptance/testdata/%.feature.ast.json

acceptance/testdata/%.feature.errors: $(GHERKIN_BASE)/testdata/%.feature $(GHERKIN_BASE)/testdata/%.feature.errors
	mkdir -p `dirname $@`
	! bin/gherkin-generate-ast $< 2> $@
	diff --unified $<.errors $@
.DELETE_ON_ERROR: acceptance/testdata/%.feature.errors

parser.go: $(GHERKIN_BASE)/gherkin.berp parser.go.razor $(GHERKIN_BASE)/bin/berp.exe
	mono $(GHERKIN_BASE)/bin/berp.exe -g $(GHERKIN_BASE)/gherkin.berp -t parser.go.razor -o $@
	# Remove BOM
	tail -c +4 $@ > $@.nobom
	mv $@.nobom $@

dialects_builtin.go: $(GHERKIN_BASE)/dialects.json
	cat $^ | jq -f $@.jq -r -c > $@

LICENSE: $(GHERKIN_BASE)/LICENSE
	cp $< $@

clean:
	rm -rf .compared .built acceptance bin/ parser.go dialects_builtin.go
.PHONY: clean

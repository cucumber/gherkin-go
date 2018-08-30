package gherkin

import (
	"github.com/cucumber/cucumber-messages-go"
	"strings"
)

type AstBuilder interface {
	Builder
	GetGherkinDocument() *messages.GherkinDocument
}

type astBuilder struct {
	stack    []*astNode
	comments []*messages.Comment
	uri      string
}

func (t *astBuilder) Reset() {
	t.comments = []*messages.Comment{}
	t.stack = []*astNode{}
	t.push(newAstNode(RuleTypeNone))
}

func (t *astBuilder) GetGherkinDocument() *messages.GherkinDocument {
	res := t.currentNode().getSingle(RuleTypeGherkinDocument)
	if val, ok := res.(*messages.GherkinDocument); ok {
		return val
	}
	return nil
}

type astNode struct {
	ruleType RuleType
	subNodes map[RuleType][]interface{}
}

func (a *astNode) add(rt RuleType, obj interface{}) {
	a.subNodes[rt] = append(a.subNodes[rt], obj)
}

func (a *astNode) getSingle(rt RuleType) interface{} {
	if val, ok := a.subNodes[rt]; ok {
		for i := range val {
			return val[i]
		}
	}
	return nil
}

func (a *astNode) getItems(rt RuleType) []interface{} {
	var res []interface{}
	if val, ok := a.subNodes[rt]; ok {
		for i := range val {
			res = append(res, val[i])
		}
	}
	return res
}

func (a *astNode) getToken(tt TokenType) *Token {
	if val, ok := a.getSingle(tt.RuleType()).(*Token); ok {
		return val
	}
	return nil
}

func (a *astNode) getTokens(tt TokenType) []*Token {
	var items = a.getItems(tt.RuleType())
	var tokens []*Token
	for i := range items {
		if val, ok := items[i].(*Token); ok {
			tokens = append(tokens, val)
		}
	}
	return tokens
}

func (t *astBuilder) currentNode() *astNode {
	if len(t.stack) > 0 {
		return t.stack[len(t.stack)-1]
	}
	return nil
}

func newAstNode(rt RuleType) *astNode {
	return &astNode{
		ruleType: rt,
		subNodes: make(map[RuleType][]interface{}),
	}
}

func NewAstBuilder(uri string) AstBuilder {
	builder := new(astBuilder)
	builder.uri = uri
	builder.comments = []*messages.Comment{}
	builder.push(newAstNode(RuleTypeNone))
	return builder
}

func (t *astBuilder) push(n *astNode) {
	t.stack = append(t.stack, n)
}

func (t *astBuilder) pop() *astNode {
	x := t.stack[len(t.stack)-1]
	t.stack = t.stack[:len(t.stack)-1]
	return x
}

func (t *astBuilder) Build(tok *Token) (bool, error) {
	if tok.Type == TokenTypeComment {
		comment := &messages.Comment{
			Location: astLocation(tok),
			Text:     tok.Text,
		}
		t.comments = append(t.comments, comment)
	} else {
		t.currentNode().add(tok.Type.RuleType(), tok)
	}
	return true, nil
}
func (t *astBuilder) StartRule(r RuleType) (bool, error) {
	t.push(newAstNode(r))
	return true, nil
}
func (t *astBuilder) EndRule(r RuleType) (bool, error) {
	node := t.pop()
	transformedNode, err := t.transformNode(node)
	t.currentNode().add(node.ruleType, transformedNode)
	return true, err
}

func (t *astBuilder) transformNode(node *astNode) (interface{}, error) {
	switch node.ruleType {

	case RuleTypeStep:
		stepLine := node.getToken(TokenTypeStepLine)

		step := &messages.Step{
			Location: astLocation(stepLine),
			Keyword:  stepLine.Keyword,
			Text:     stepLine.Text,
		}
		dataTable := node.getSingle(RuleTypeDataTable)
		if dataTable != nil {
			step.Argument = &messages.Step_DataTable{DataTable: dataTable.(*messages.DataTable)}
		} else {
			docString := node.getSingle(RuleTypeDocString)
			if docString != nil {
				step.Argument = &messages.Step_DocString{DocString: docString.(*messages.DocString)}
			}
		}

		return step, nil

	case RuleTypeDocString:
		separatorToken := node.getToken(TokenTypeDocStringSeparator)
		lineTokens := node.getTokens(TokenTypeOther)
		var text string
		for i := range lineTokens {
			if i > 0 {
				text += "\n"
			}
			text += lineTokens[i].Text
		}
		ds := &messages.DocString{
			Location:    astLocation(separatorToken),
			ContentType: separatorToken.Text,
			Content:     text,
			Delimiter:   separatorToken.Keyword,
		}
		return ds, nil

	case RuleTypeDataTable:
		rows, err := astTableRows(node)
		dt := &messages.DataTable{
			Location: rows[0].Location,
			Rows:     rows,
		}
		return dt, err

	case RuleTypeBackground:
		backgroundLine := node.getToken(TokenTypeBackgroundLine)
		description, _ := node.getSingle(RuleTypeDescription).(string)
		bg := &messages.Background{
			Location:    astLocation(backgroundLine),
			Keyword:     backgroundLine.Keyword,
			Name:        backgroundLine.Text,
			Description: description,
			Steps:       astSteps(node),
		}
		return bg, nil

	case RuleTypeScenarioDefinition:
		tags := astTags(node)
		scenarioNode, _ := node.getSingle(RuleTypeScenario).(*astNode)

		scenarioLine := scenarioNode.getToken(TokenTypeScenarioLine)
		description, _ := scenarioNode.getSingle(RuleTypeDescription).(string)
		sc := &messages.Scenario{
			Tags:        tags,
			Location:    astLocation(scenarioLine),
			Keyword:     scenarioLine.Keyword,
			Name:        scenarioLine.Text,
			Description: description,
			Steps:       astSteps(scenarioNode),
			Examples:    astExamples(scenarioNode),
		}

		return sc, nil

	case RuleTypeExamplesDefinition:
		tags := astTags(node)
		examplesNode, _ := node.getSingle(RuleTypeExamples).(*astNode)
		examplesLine := examplesNode.getToken(TokenTypeExamplesLine)
		description, _ := examplesNode.getSingle(RuleTypeDescription).(string)
		examplesTable := examplesNode.getSingle(RuleTypeExamplesTable)

		// TODO: Is this mutation style ok?
		ex := &messages.Examples{}
		ex.Tags = tags
		ex.Location = astLocation(examplesLine)
		ex.Keyword = examplesLine.Keyword
		ex.Name = examplesLine.Text
		ex.Description = description
		ex.TableHeader = nil
		ex.TableBody = nil
		if examplesTable != nil {
			allRows, _ := examplesTable.([]*messages.TableRow)
			ex.TableHeader = allRows[0]
			ex.TableBody = allRows[1:]
		}
		return ex, nil

	case RuleTypeExamplesTable:
		allRows, err := astTableRows(node)
		return allRows, err

	case RuleTypeDescription:
		lineTokens := node.getTokens(TokenTypeOther)
		// Trim trailing empty lines
		end := len(lineTokens)
		for end > 0 && strings.TrimSpace(lineTokens[end-1].Text) == "" {
			end--
		}
		var desc []string
		for i := range lineTokens[0:end] {
			desc = append(desc, lineTokens[i].Text)
		}
		return strings.Join(desc, "\n"), nil

	case RuleTypeFeature:
		header, ok := node.getSingle(RuleTypeFeatureHeader).(*astNode)
		if !ok {
			return nil, nil
		}
		tags := astTags(header)
		featureLine := header.getToken(TokenTypeFeatureLine)
		if featureLine == nil {
			return nil, nil
		}

		var children []*messages.FeatureChild
		background, _ := node.getSingle(RuleTypeBackground).(*messages.Background)
		if background != nil {
			children = append(children, &messages.FeatureChild{
				Value: &messages.FeatureChild_Background{Background: background},
			})
		}
		scenarios := node.getItems(RuleTypeScenarioDefinition)
		for i := range scenarios {
			scenario := scenarios[i].(*messages.Scenario)
			children = append(children, &messages.FeatureChild{
				Value: &messages.FeatureChild_Scenario{Scenario: scenario},
			})
		}
		rules := node.getItems(RuleTypeRule)
		for i := range rules {
			rule := rules[i].(*messages.Rule)
			children = append(children, &messages.FeatureChild{
				Value: &messages.FeatureChild_Rule{Rule: rule},
			})
		}

		description, _ := header.getSingle(RuleTypeDescription).(string)

		feat := &messages.Feature{}
		feat.Tags = tags
		feat.Location = astLocation(featureLine)
		feat.Language = featureLine.GherkinDialect
		feat.Keyword = featureLine.Keyword
		feat.Name = featureLine.Text
		feat.Description = description
		feat.Children = children
		return feat, nil

	case RuleTypeRule:
		header, ok := node.getSingle(RuleTypeRuleHeader).(*astNode)
		if !ok {
			return nil, nil
		}
		ruleLine := header.getToken(TokenTypeRuleLine)
		if ruleLine == nil {
			return nil, nil
		}

		var children []*messages.RuleChild
		background, _ := node.getSingle(RuleTypeBackground).(*messages.Background)
		if background != nil {
			children = append(children, &messages.RuleChild{
				Value: &messages.RuleChild_Background{Background: background},
			})
		}
		scenarios := node.getItems(RuleTypeScenarioDefinition)
		for i := range scenarios {
			scenario := scenarios[i].(*messages.Scenario)
			children = append(children, &messages.RuleChild{
				Value: &messages.RuleChild_Scenario{Scenario: scenario},
			})
		}

		description, _ := header.getSingle(RuleTypeDescription).(string)

		rule := &messages.Rule{}
		rule.Location = astLocation(ruleLine)
		rule.Keyword = ruleLine.Keyword
		rule.Name = ruleLine.Text
		rule.Description = description
		rule.Children = children
		return rule, nil

	case RuleTypeGherkinDocument:
		feature, _ := node.getSingle(RuleTypeFeature).(*messages.Feature)

		doc := &messages.GherkinDocument{}
		if feature != nil {
			doc.Feature = feature
		}
		doc.Comments = t.comments
		doc.Uri = t.uri
		return doc, nil
	}
	return node, nil
}

func astLocation(t *Token) *messages.Location {
	return &messages.Location{
		Line:   uint32(t.Location.Line),
		Column: uint32(t.Location.Column),
	}
}

func astTableRows(t *astNode) (rows []*messages.TableRow, err error) {
	rows = []*messages.TableRow{}
	tokens := t.getTokens(TokenTypeTableRow)
	for i := range tokens {
		row := &messages.TableRow{
			Location: astLocation(tokens[i]),
			Cells:    astTableCells(tokens[i]),
		}
		rows = append(rows, row)
	}
	err = ensureCellCount(rows)
	return
}

func ensureCellCount(rows []*messages.TableRow) error {
	if len(rows) <= 1 {
		return nil
	}
	cellCount := len(rows[0].Cells)
	for i := range rows {
		if cellCount != len(rows[i].Cells) {
			return &parseError{"inconsistent cell count within the table", &Location{
				Line:   int(rows[i].Location.Line),
				Column: int(rows[i].Location.Column),
			}}
		}
	}
	return nil
}

func astTableCells(t *Token) (cells []*messages.TableCell) {
	cells = []*messages.TableCell{}
	for i := range t.Items {
		item := t.Items[i]
		cell := &messages.TableCell{}
		cell.Location = &messages.Location{
			Line:   uint32(t.Location.Line),
			Column: uint32(item.Column),
		}
		cell.Value = item.Text
		cells = append(cells, cell)
	}
	return
}

func astSteps(t *astNode) (steps []*messages.Step) {
	steps = []*messages.Step{}
	tokens := t.getItems(RuleTypeStep)
	for i := range tokens {
		step, _ := tokens[i].(*messages.Step)
		steps = append(steps, step)
	}
	return
}

func astExamples(t *astNode) (examples []*messages.Examples) {
	examples = []*messages.Examples{}
	tokens := t.getItems(RuleTypeExamplesDefinition)
	for i := range tokens {
		example, _ := tokens[i].(*messages.Examples)
		examples = append(examples, example)
	}
	return
}

func astTags(node *astNode) (tags []*messages.Tag) {
	tags = []*messages.Tag{}
	tagsNode, ok := node.getSingle(RuleTypeTags).(*astNode)
	if !ok {
		return
	}
	tokens := tagsNode.getTokens(TokenTypeTagLine)
	for i := range tokens {
		token := tokens[i]
		for k := range token.Items {
			item := token.Items[k]
			tag := &messages.Tag{}
			tag.Location = &messages.Location{
				Line:   uint32(token.Location.Line),
				Column: uint32(item.Column),
			}
			tag.Name = item.Text
			tags = append(tags, tag)
		}
	}
	return
}

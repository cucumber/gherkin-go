package gherkin

import (
	"bytes"
	"github.com/cucumber/cucumber-messages-go/v2"
	gio "github.com/gogo/protobuf/io"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMessagesWithStdin(t *testing.T) {
	stdin := &bytes.Buffer{}
	writer := gio.NewDelimitedWriter(stdin)

	gherkin := `Feature: Minimal

  Scenario: a
    Given a

  Scenario: b
    Given b
`

	wrapper := &messages.Wrapper{
		Message: &messages.Wrapper_Source{
			Source: &messages.Source{
				Uri:  "features/test.feature",
				Data: gherkin,
				Media: &messages.Media{
					Encoding:    "UTF-8",
					ContentType: "text/x.cucumber.gherkin+plain",
				},
			},
		},
	}

	writer.WriteMsg(wrapper)

	wrappers, err := Messages(
		nil,
		stdin,
		"en",
		true,
		true,
		true,
		nil,
		false,
		true,
	)

	require.NoError(t, err)
	require.Equal(t, 12, len(wrappers))
}

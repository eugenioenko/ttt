package i18n

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
)

//go:embed en.json
var englishJSON []byte

var english = func() map[string]string {
	var messages map[string]string
	if err := json.Unmarshal(englishJSON, &messages); err != nil {
		panic(err)
	}
	return messages
}()

type Args map[string]any

type Message struct{ id string }

func Key(id string) Message { return Message{id: id} }

func (m Message) Text() string { return Text(m.id) }

var active atomic.Pointer[Catalog]

func Text(id string) string {
	if catalog := active.Load(); catalog != nil {
		if message, ok := catalog.messages[id]; ok {
			return message
		}
	}
	if message, ok := english[id]; ok {
		return message
	}
	return id
}

func SetCatalog(catalog *Catalog) { active.Store(catalog) }

func Format(id string, args Args) string {
	return interpolate(Text(id), args)
}

func interpolate(message string, args Args) string {
	var result strings.Builder
	for {
		start := strings.IndexByte(message, '{')
		if start < 0 {
			result.WriteString(message)
			return result.String()
		}
		end := strings.IndexByte(message[start:], '}')
		if end < 0 {
			result.WriteString(message)
			return result.String()
		}
		end += start
		result.WriteString(message[:start])
		if value, ok := args[message[start+1:end]]; ok {
			fmt.Fprint(&result, value)
		} else {
			result.WriteString(message[start : end+1])
		}
		message = message[end+1:]
	}
}

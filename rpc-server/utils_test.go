package main

import (
	"errors"
	"testing"

	"github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc"
	"github.com/stretchr/testify/assert"
)

func TestGetSenderReceiver(t *testing.T) {
	tests := []struct {
		name     string
		chatId   string
		sender   string
		receiver string
	}{
		{
			"sender first receiver second",
			"alpha:beta",
			"alpha",
			"beta",
		},
		{
			"receiver first sender second",
			"alpha:beta",
			"beta",
			"alpha",
		},
		{
			"same sender receiver",
			"alpha:alpha",
			"alpha",
			"alpha",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			message := rpc.NewMessage()
			message.SetChat(testCase.chatId)
			message.SetSender(testCase.sender)
			sender, receiver := GetSenderReceiver(message)
			assert.True(t, sender == testCase.sender)
			assert.True(t, receiver == testCase.receiver)
		})
	}
}

func TestNormaliseChatKey(t *testing.T) {
	tests := []struct {
		name     string
		chatKey  string
		expected string
	}{
		{
			"a:a same",
			"a:a",
			"a:a",
		},
		{
			"a:b sorted",
			"a:b",
			"a:b",
		},
		{
			"a:b unsorted",
			"b:a",
			"a:b",
		},
		{
			"alpha:beta unsorted",
			"beta:alpha",
			"alpha:beta",
		},
		{
			"alpha:beta sorted",
			"alpha:beta",
			"alpha:beta",
		},
		{
			"alpha:zulu unsorted",
			"zulu:alpha",
			"alpha:zulu",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			m := rpc.NewMessage()
			m.SetChat(testCase.chatKey)
			assert.True(t, GetNormalisedChatIDFromMessage(m) == testCase.expected)
		})
	}
}

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		message *rpc.Message
		output  error
	}{
		{
			"valid message",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "hi",
				Sender:   "a",
				SendTime: 1,
			},
			nil,
		},
		{
			"valid message with second entry as sender",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "hi",
				Sender:   "b",
				SendTime: 1,
			},
			nil,
		},
		{
			"valid message with spaces prefix and suffix space",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "   hi    ",
				Sender:   "a",
				SendTime: 1,
			},
			nil,
		},
		{
			"invalid chat id with extra entry",
			&rpc.Message{
				Chat:     "a:b:c",
				Text:     "hi",
				Sender:   "a",
				SendTime: 1,
			},
			invalidChatID,
		},
		{
			"invalid chat id with empty first entry",
			&rpc.Message{
				Chat:     ":b",
				Text:     "hi",
				Sender:   "a",
				SendTime: 1,
			},
			invalidChatID,
		},
		{
			"invalid chat id with empty second entry",
			&rpc.Message{
				Chat:     "a:",
				Text:     "hi",
				Sender:   "a",
				SendTime: 1,
			},
			invalidChatID,
		},
		{
			"all spaces text",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "       ",
				Sender:   "a",
				SendTime: 1,
			},
			invalidMessage,
		},
		{
			"empty text",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "",
				Sender:   "a",
				SendTime: 1,
			},
			invalidMessage,
		},
		{
			"invalid sender",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "hi",
				Sender:   "c",
				SendTime: 1,
			},
			invalidSender,
		},
		{
			"invalid send time",
			&rpc.Message{
				Chat:     "a:b",
				Text:     "hi",
				Sender:   "a",
				SendTime: -1,
			},
			invalidSendTime,
		},
		{ // should return first error
			"all invalid",
			&rpc.Message{
				Chat:     "",
				Text:     "",
				Sender:   "c",
				SendTime: -1,
			},
			invalidChatID,
		},
		{
			"nil message",
			nil,
			invalidMessage,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateMessage(testCase.message)
			assert.True(t, errors.Is(err, testCase.output))
		})
	}
}

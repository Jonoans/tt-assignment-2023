package main

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/TikTokTechImmersion/assignment_demo_2023/rpc-server/kitex_gen/rpc"
	"golang.org/x/exp/constraints"
)

var (
	invalidMessage  = errors.New("invalid message")
	invalidChatID   = errors.New("invalid chat id")
	invalidSender   = errors.New("invalid sender")
	invalidSendTime = errors.New("invalid send time")
)

func GetSenderReceiver(message *rpc.Message) (string, string) {
	splitChatID := strings.Split(message.GetChat(), ":")
	receiver := splitChatID[0]
	if message.GetSender() == receiver {
		receiver = splitChatID[1]
	}
	return message.GetSender(), receiver
}

func GetNormalisedChatIDFromMessage(message *rpc.Message) string {
	return GetNormalisedChatID(message.GetChat())
}

func GetNormalisedChatID(chatId string) string {
	splitChatID := strings.Split(chatId, ":")
	sort.Strings(splitChatID)
	return strings.Join(splitChatID, ":")
}

func Min[V constraints.Ordered](values ...V) V {
	if len(values) == 0 {
		panic("No values provided to Min")
	}

	result := values[0]
	for _, value := range values {
		if value < result {
			result = value
		}
	}

	return result
}

func ValidateChatID(chatId string) error {
	splitChatID := strings.Split(chatId, ":")
	if len(splitChatID) != 2 {
		return invalidChatID
	}

	if splitChatID[0] == "" || splitChatID[1] == "" {
		return invalidChatID
	}

	return nil
}

func ValidateMessage(message *rpc.Message) error {
	if message == nil {
		return invalidMessage
	}

	if err := ValidateChatID(message.GetChat()); err != nil {
		return err
	}

	splitChatID := strings.Split(message.GetChat(), ":")
	if message.GetSender() != splitChatID[0] && message.GetSender() != splitChatID[1] {
		return invalidSender
	}

	message.SetText(strings.TrimSpace(message.GetText()))
	if message.GetText() == "" {
		return invalidMessage
	}

	if message.GetSendTime() < 0 {
		return invalidSendTime
	}

	return nil
}

func GetTimeNow() time.Time {
    return time.Now().UTC()
}

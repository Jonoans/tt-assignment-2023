package main

import (
	"context"
	"log"
	"time"

	"github.com/TikTokTechImmersion/assignment_demo_2023/http-server/kitex_gen/rpc"
	"github.com/TikTokTechImmersion/assignment_demo_2023/http-server/kitex_gen/rpc/imservice"
	"github.com/TikTokTechImmersion/assignment_demo_2023/http-server/proto_gen/api"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/kitex/client"
	etcd "github.com/kitex-contrib/registry-etcd"
)

var cli imservice.Client

type PullResponseRest struct {
	Messages   []*api.Message `protobuf:"bytes,1,rep,name=messages,proto3" json:"messages"`
	HasMore    bool           `protobuf:"varint,2,opt,name=has_more,json=hasMore,proto3" json:"has_more"`                    // if true, can use next_cursor to pull the next page of messages
	NextCursor int64          `protobuf:"varint,3,opt,name=next_cursor,json=nextCursor,proto3" json:"next_cursor,omitempty"` // starting position of next page, inclusively
}

func main() {
	r, err := etcd.NewEtcdResolver([]string{"etcd:2379"})
	if err != nil {
		log.Fatal(err)
	}
	cli = imservice.MustNewClient("demo.rpc.server",
		client.WithResolver(r),
		client.WithRPCTimeout(1*time.Second),
	)

	h := server.Default(server.WithHostPorts("0.0.0.0:8080"))

	h.GET("/ping", func(c context.Context, ctx *app.RequestContext) {
		ctx.JSON(consts.StatusOK, utils.H{"message": "pong"})
	})

	h.POST("/api/send", sendMessage)
	h.GET("/api/pull", pullMessage)

	h.Spin()
}

func sendMessage(ctx context.Context, c *app.RequestContext) {
	var req api.SendRequest
	err := c.Bind(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, "Failed to parse request body: %v", err)
		return
	}
	resp, err := cli.Send(ctx, &rpc.SendRequest{
		Message: &rpc.Message{
			Chat:     req.Chat,
			Text:     req.Text,
			Sender:   req.Sender,
			SendTime: time.Now().UTC().UnixMicro(),
		},
	})
	if err != nil {
		c.String(consts.StatusInternalServerError, err.Error())
	} else if resp.Code != 0 {
		c.String(consts.StatusInternalServerError, resp.Msg)
	} else {
		c.Status(consts.StatusOK)
	}
}

func pullMessage(ctx context.Context, c *app.RequestContext) {
	var req api.PullRequest
	err := c.Bind(&req)
	if err != nil {
		c.String(consts.StatusBadRequest, "Failed to parse request body: %v", err)
		return
	}

	resp, err := cli.Pull(ctx, &rpc.PullRequest{
		Chat:    req.Chat,
		Cursor:  req.Cursor,
		Limit:   req.Limit,
		Reverse: &req.Reverse,
	})
	if err != nil {
		c.String(consts.StatusInternalServerError, err.Error())
		return
	} else if resp.Code != 0 {
		c.String(consts.StatusInternalServerError, resp.Msg)
		return
	}
	messages := make([]*api.Message, 0, len(resp.Messages))
	for _, msg := range resp.Messages {
		messages = append(messages, &api.Message{
			Chat:     msg.Chat,
			Text:     msg.Text,
			Sender:   msg.Sender,
			SendTime: msg.SendTime,
		})
	}
	c.JSON(consts.StatusOK, &PullResponseRest{
		Messages:   messages,
		HasMore:    resp.GetHasMore(),
		NextCursor: resp.GetNextCursor(),
	})
}

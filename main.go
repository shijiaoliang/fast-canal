/**
 ********************************************************************************************
 * Copyright (c)  fast-canal
 * Created by fast-canal.
 * User: shijl
 * Date: 2020/09/01
 * Time: 11:18
 ********************************************************************************************
 */

package main

import (
	"time"
	"github.com/gogf/gf/frame/g"
	"github.com/gogf/gf/net/ghttp"
	"github.com/gogf/gf/os/grpool"
	"github.com/siddontang/go-mysql/canal"
	"github.com/siddontang/go-mysql/mysql"
)

var httpClient *ghttp.Client

func init() {
	//100毫秒超时
	httpClient = ghttp.NewClient().SetTimeout(100 * time.Millisecond).SetContentType("application/json")

	//log
	g.Log().SetAsync(true)
}

type eventData struct {
	Database string          `json:"database"`
	Table    string          `json:"table"`
	Action   string          `json:"action"`
	PkValues []interface{}   `json:"pk_values"`
	Rows     [][]interface{} `json:"rows"`
}

type myEventHandler struct {
	canal.DummyEventHandler
}

func (h *myEventHandler) OnRow(e *canal.RowsEvent) error {
	var pkValues []interface{}
	for i, item := range e.Rows {
		if e.Action == "update" && i%2 == 0 {
			continue
		}

		pkValue, _ := e.Table.GetPKValues(item)
		pkValues = append(pkValues, pkValue[0])
	}

	ed := &eventData{
		Database: e.Table.Schema,
		Table:    e.Table.Name,
		Action:   e.Action,
		PkValues: pkValues,
		Rows:     e.Rows,
	}

	//log ed
	g.Log().Info(ed)

	//协程池复用-http通知
	grpool.Add(func() {
		urls := g.Cfg().GetStrings("notify.Urls")
		for _, url := range urls {
			httpClient.PostContent(url, ed)
		}
	})

	return nil
}

func (h *myEventHandler) String() string {
	return "myEventHandler"
}

func main() {
	//cfg
	cfg := canal.NewDefaultConfig()
	cfg.Addr = g.Cfg().GetString("source.Addr")
	cfg.User = g.Cfg().GetString("source.User")
	cfg.Password = g.Cfg().GetString("source.Password")
	cfg.Dump.ExecutionPath = ""
	cfg.IncludeTableRegex = g.Cfg().GetStrings("source.IncludeTableRegex")

	//canal
	c, _ := canal.NewCanal(cfg)
	c.SetEventHandler(&myEventHandler{})

	//startPos
	var startPos mysql.Position
	var err error
	startPosName := g.Cfg().GetString("source.startPos.Name")
	startPosPos := g.Cfg().GetUint32("source.startPos.Pos")
	if startPosName != "" && startPosPos > 0 {
		startPos = mysql.Position{
			Name: startPosName,
			Pos:  startPosPos,
		}
	} else {
		startPos, err = c.GetMasterPos()
		if err != nil {
			g.Log().Printf("getMasterPos err %v", err)
			return
		}
	}

	//run
	err = c.RunFrom(startPos)
	if err != nil {
		g.Log().Printf("start fast-canal err %v", err)
	}
}

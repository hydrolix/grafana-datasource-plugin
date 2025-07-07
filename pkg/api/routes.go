package api

import (
	"encoding/json"
	"errors"
	clickhouse "github.com/hydrolix/plugin/pkg/parser"
	"net/http"
)

func AST(rw http.ResponseWriter, req *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			sendError(rw, errors.New("Unknown Error"))
		}
	}()
	var astRequest ASTRequest
	if err := json.NewDecoder(req.Body).Decode(&astRequest); err != nil {
		sendError(rw, err)
		return
	}

	body, err := clickhouse.NewParser(astRequest.Data.Query).ParseStmts()
	if err != nil {
		sendError(rw, err)
		return

	}

	rw.WriteHeader(http.StatusOK)
	marshal, err := json.Marshal(ASTResponse{
		false,
		"",
		body,
	})
	_, err = rw.Write(marshal)
}

func sendError(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusOK)
	marshal, _ := json.Marshal(ASTResponse{
		true,
		err.Error(),
		nil,
	})
	_, err = rw.Write(marshal)
	return
}

func Routes() map[string]func(http.ResponseWriter, *http.Request) {
	return map[string]func(http.ResponseWriter, *http.Request){
		"/ast": AST,
	}
}

type ASTRequest struct {
	Data struct {
		Query string `json:"query"`
	} `json:"data"`
}
type ASTResponse struct {
	Error        bool              `json:"error"`
	ErrorMessage string            `json:"error_message"`
	Data         []clickhouse.Expr `json:"data"`
}

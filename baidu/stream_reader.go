package baidu

import (
	"bufio"
	"errors"
	"fmt"
	utils "github.com/liudding/go-llm-api/internal"
	"github.com/liudding/go-llm-api/internal/sse"
	"io"
	"net/http"
)

var (
	ErrTooManyEmptyStreamMessages = errors.New("stream has sent too many empty messages")
)

type streamReader struct {
	emptyMessagesLimit uint
	isFinished         bool

	reader         *sse.EventStreamReader
	response       *http.Response
	errAccumulator utils.ErrorAccumulator
	unmarshaler    utils.Unmarshaler
}

func newStreamReader(response *http.Response) *streamReader {
	reader := sse.NewEventStreamReader(bufio.NewReader(response.Body), 1024)

	return &streamReader{
		reader:         reader,
		response:       response,
		errAccumulator: utils.NewErrorAccumulator(),
		unmarshaler:    &utils.JSONUnmarshaler{},
	}
}

func (stream *streamReader) Recv() (response ChatCompletionResponse, err error) {
	event, err := stream.reader.Recv()
	if err != nil {
		return
	}

	if stream.isFinished {
		err = io.EOF
		return
	}

	if event.Data == nil {
		unmarshalErr := stream.unmarshaler.Unmarshal(event.Other, &response)
		if unmarshalErr != nil {
			return response, unmarshalErr
		}

		if response.ErrorCode > 0 {
			return response, fmt.Errorf("[%d][%s]", response.ErrorCode, response.ErrorMsg)
		}
		return
	}

	unmarshalErr := stream.unmarshaler.Unmarshal(event.Data, &response)
	if unmarshalErr != nil {
		return response, unmarshalErr
	}

	if response.IsEnd {
		stream.isFinished = true
		return
	}

	return
}

func (stream *streamReader) unmarshalError() (errResp *ErrorResponse) {
	errBytes := stream.errAccumulator.Bytes()
	if len(errBytes) == 0 {
		return
	}

	err := stream.unmarshaler.Unmarshal(errBytes, &errResp)
	if err != nil {
		errResp = nil
	}

	return
}

func (stream *streamReader) Close() {
	stream.response.Body.Close()
}

package protocol

import (
	"encoding/base64"
	"fmt"
	"sort"
	"testing"
)

func BenchmarkPaginationLimit(b *testing.B) {
	list := getTools(10000)
	for i := 0; i < b.N; i++ {
		PaginationLimit[Tool](list, "dG9vbDMz", 10)
	}
}

func BenchmarkPaginationLimitForTool(b *testing.B) {
	list := getTools(10000)
	for i := 0; i < b.N; i++ {
		PaginationLimitForTool(list, "dG9vbDMz", 10)
	}
}

func getTools(length int) []Tool {
	list := make([]Tool, 0, 10000)
	for i := 0; i < length; i++ {
		list = append(list, Tool{
			Name:        fmt.Sprintf("tool%d", i),
			Description: fmt.Sprintf("tool%d", i),
		})
	}
	return list
}

func PaginationLimitForTool(allElements []Tool, cursor Cursor, limit int) ([]Tool, Cursor, error) {
	startPos := 0
	if cursor != "" {
		c, err := base64.StdEncoding.DecodeString(string(cursor))
		if err != nil {
			return nil, "", err
		}
		cString := string(c)
		startPos = sort.Search(len(allElements), func(i int) bool {
			nc := allElements[i].Name
			return nc > cString
		})
	}
	endPos := len(allElements)
	if len(allElements) > startPos+limit {
		endPos = startPos + limit
	}
	elementsToReturn := allElements[startPos:endPos]
	// set the next cursor
	nextCursor := func() Cursor {
		if len(elementsToReturn) < limit {
			return ""
		}
		element := elementsToReturn[len(elementsToReturn)-1]
		nc := element.Name
		toString := base64.StdEncoding.EncodeToString([]byte(nc))
		return Cursor(toString)
	}()
	return elementsToReturn, nextCursor, nil
}

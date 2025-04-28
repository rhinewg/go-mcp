package protocol

import (
	"encoding/base64"
	"reflect"
	"sort"
)

// Cursor is an opaque token used to represent a cursor for pagination.
type Cursor string

func PaginationLimit[T any](allElements []T, cursor Cursor, limit int) ([]T, Cursor, error) {
	startPos := 0
	if cursor != "" {
		c, err := base64.StdEncoding.DecodeString(string(cursor))
		if err != nil {
			return nil, "", err
		}
		cString := string(c)
		startPos = sort.Search(len(allElements), func(i int) bool {
			val := reflect.ValueOf(allElements[i])
			var nc string
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			nc = val.FieldByName("Name").String()
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
		if len(elementsToReturn) >= limit {
			// fmt.Printf("-=====%+v\n", elementsToReturn[len(elementsToReturn)-1])
			element := elementsToReturn[len(elementsToReturn)-1]
			val := reflect.ValueOf(element)
			var nc string
			if val.Kind() == reflect.Ptr {
				val = val.Elem()
			}
			nc = val.FieldByName("Name").String()
			toString := base64.StdEncoding.EncodeToString([]byte(nc))
			return Cursor(toString)
		}
		return ""
	}()
	return elementsToReturn, nextCursor, nil
}

// PaginatedRequest represents a request that supports pagination
type PaginatedRequest struct {
	Cursor Cursor `json:"cursor,omitempty"`
}

// PaginatedResult represents a response that supports pagination
type PaginatedResult struct {
	NextCursor Cursor `json:"nextCursor,omitempty"`
}

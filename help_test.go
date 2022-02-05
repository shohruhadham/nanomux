// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// --------------------------------------------------

func lineInfo(skipCount int) string {
	if pc, _, _, ok := runtime.Caller(skipCount); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			var strb strings.Builder
			strb.WriteString("[")
			var _, line = fn.FileLine(pc)
			strb.WriteString(strconv.Itoa(line))
			strb.WriteByte(' ')

			var fnName = strings.TrimPrefix(fn.Name(), "Test")
			var idx = strings.LastIndexByte(fnName, '.')
			if idx < 0 {
				strb.WriteString(fnName)
			} else {
				strb.WriteString(fnName[idx+1:])
			}

			strb.WriteString("] ")
			return strb.String()
		}
	}

	return ""
}

func testPanicker(t *testing.T, wantErr bool, fn func()) {
	var li = lineInfo(2)
	defer func() {
		var v = recover()
		if v == nil {
			return
		}

		var err, validErr = v.(error)
		if !validErr {
			panic(v)
		}

		if (err != nil) != wantErr {
			t.Fatalf("%s err = %v, want err %t", li, err, wantErr)
		}
	}()

	fn()
}

func checkPanic(t *testing.T, wantErr bool) {
	var v = recover()
	if v == nil {
		return
	}

	var err, validErr = v.(error)
	if !validErr {
		panic(v)
	}

	if (err != nil) != wantErr {
		t.Fatalf("%s err = %v, want err %t", lineInfo(2), err, wantErr)
	}
}

// func checkErr(t *testing.T, err error, wantErr bool) {
// 	t.Helper()
// 	if (err != nil) != wantErr {
// 		t.Fatalf("%s err = %v, want err  %t", lineInfo(2), err, wantErr)
// 	}
// }

// --------------------------------------------------

// implType is usef in other test files too.
type implType struct{}

func (rht *implType) HandleGet(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *implType) HandlePost(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *implType) HandleCustom(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *implType) HandleNotAllowedMethod(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *implType) SomeMethod(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *implType) SomeOtherMethod(*Args) bool {
	return true
}

func (rht *implType) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

const rhTypeHTTPMethods = "get post custom"

// --------------------------------------------------

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

func testPanicker(t *testing.T, wantPanic bool, fn func()) {
	t.Helper()
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

		if (err != nil) != wantPanic {
			t.Fatalf("%s panic err = %v, want panic %t", li, err, wantPanic)
		}
	}()

	fn()
}

func testPanickerValue(
	t *testing.T,
	wantPanic bool,
	wantValue interface{},
	fn func() interface{},
) {
	t.Helper()
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

		if (err != nil) != wantPanic {
			t.Fatalf("%s panic err = %v, want panic %t", li, err, wantPanic)
		}
	}()

	var value = fn()
	if value != wantValue {
		t.Fatalf("%s value = %v, want %v", li, value, wantValue)
	}
}

func checkErr(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if (err != nil) != wantErr {
		t.Fatalf("%s err = %v, want err %t", lineInfo(2), err, wantErr)
	}
}

func checkValue(t *testing.T, value, wantValue interface{}) {
	t.Helper()
	// The following doesn't work when the value is a nil pointer.
	if value != wantValue {
		t.Fatalf("%s value = %v, want %v", lineInfo(2), value, wantValue)
	}
}

// --------------------------------------------------

// _ImplType is usef in other test files too.
type _ImplType struct{}

func (rht *_ImplType) HandleGet(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *_ImplType) HandlePost(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *_ImplType) HandleCustom(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *_ImplType) HandleNotAllowedMethod(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *_ImplType) SomeMethod(
	http.ResponseWriter,
	*http.Request,
	*Args,
) bool {
	return true
}

func (rht *_ImplType) SomeOtherMethod(*Args) bool {
	return true
}

func (rht *_ImplType) ServeHTTP(w http.ResponseWriter, r *http.Request) {}

const rhTypeHTTPMethods = "get post custom"

// --------------------------------------------------

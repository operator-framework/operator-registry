// Code generated by counterfeiter. DO NOT EDIT.
package sqlitefakes

import (
	"context"
	"sync"

	"github.com/operator-framework/operator-registry/pkg/sqlite"
)

type FakeQuerier struct {
	QueryContextStub        func(context.Context, string, ...interface{}) (sqlite.RowScanner, error)
	queryContextMutex       sync.RWMutex
	queryContextArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 []interface{}
	}
	queryContextReturns struct {
		result1 sqlite.RowScanner
		result2 error
	}
	queryContextReturnsOnCall map[int]struct {
		result1 sqlite.RowScanner
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeQuerier) QueryContext(arg1 context.Context, arg2 string, arg3 ...interface{}) (sqlite.RowScanner, error) {
	fake.queryContextMutex.Lock()
	ret, specificReturn := fake.queryContextReturnsOnCall[len(fake.queryContextArgsForCall)]
	fake.queryContextArgsForCall = append(fake.queryContextArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 []interface{}
	}{arg1, arg2, arg3})
	stub := fake.QueryContextStub
	fakeReturns := fake.queryContextReturns
	fake.recordInvocation("QueryContext", []interface{}{arg1, arg2, arg3})
	fake.queryContextMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeQuerier) QueryContextCallCount() int {
	fake.queryContextMutex.RLock()
	defer fake.queryContextMutex.RUnlock()
	return len(fake.queryContextArgsForCall)
}

func (fake *FakeQuerier) QueryContextCalls(stub func(context.Context, string, ...interface{}) (sqlite.RowScanner, error)) {
	fake.queryContextMutex.Lock()
	defer fake.queryContextMutex.Unlock()
	fake.QueryContextStub = stub
}

func (fake *FakeQuerier) QueryContextArgsForCall(i int) (context.Context, string, []interface{}) {
	fake.queryContextMutex.RLock()
	defer fake.queryContextMutex.RUnlock()
	argsForCall := fake.queryContextArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeQuerier) QueryContextReturns(result1 sqlite.RowScanner, result2 error) {
	fake.queryContextMutex.Lock()
	defer fake.queryContextMutex.Unlock()
	fake.QueryContextStub = nil
	fake.queryContextReturns = struct {
		result1 sqlite.RowScanner
		result2 error
	}{result1, result2}
}

func (fake *FakeQuerier) QueryContextReturnsOnCall(i int, result1 sqlite.RowScanner, result2 error) {
	fake.queryContextMutex.Lock()
	defer fake.queryContextMutex.Unlock()
	fake.QueryContextStub = nil
	if fake.queryContextReturnsOnCall == nil {
		fake.queryContextReturnsOnCall = make(map[int]struct {
			result1 sqlite.RowScanner
			result2 error
		})
	}
	fake.queryContextReturnsOnCall[i] = struct {
		result1 sqlite.RowScanner
		result2 error
	}{result1, result2}
}

func (fake *FakeQuerier) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.queryContextMutex.RLock()
	defer fake.queryContextMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeQuerier) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ sqlite.Querier = new(FakeQuerier)

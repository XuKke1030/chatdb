package utility

import (
	"errors"
	"strings"
	"testing"
)

func TestSafeUserErr_Nil(t *testing.T) {
	got := SafeUserErr(nil)
	if got != "当前查询暂时未能完成，请稍后重试。" {
		t.Fatalf("unexpected nil message: %s", got)
	}
}

func TestSafeUserErr_MaxSteps(t *testing.T) {
	cases := []error{
		errors.New("GraphRunError: max steps"),
		errors.New("exceeds max steps limit"),
	}
	for _, e := range cases {
		got := SafeUserErr(e)
		if !strings.Contains(got, "步骤过多") {
			t.Fatalf("for %v got %q, want step-related message", e, got)
		}
	}
}

func TestSafeUserErr_Timeout(t *testing.T) {
	got := SafeUserErr(errors.New("context deadline exceeded"))
	if !strings.Contains(got, "超时") {
		t.Fatalf("unexpected timeout message: %s", got)
	}
}

func TestSafeUserErr_DBError_LeakPrevented(t *testing.T) {
	orig := errors.New("Error 1045 (28000): Access denied for user 'root'@'10.0.1.5' (using password: YES)")
	got := SafeUserErr(orig)
	if got == orig.Error() {
		t.Fatal("database error should not be leaked to user")
	}
}

func TestSafeUserErr_SyntaxError(t *testing.T) {
	got := SafeUserErr(errors.New("You have an error in your SQL syntax"))
	if !strings.Contains(got, "语法") {
		t.Fatalf("unexpected syntax message: %s", got)
	}
}

func TestSafeUserErr_TableNotFound(t *testing.T) {
	got := SafeUserErr(errors.New("Table 'chatdb.users' doesn't exist"))
	if !strings.Contains(got, "表不存在") {
		t.Fatalf("unexpected table message: %s", got)
	}
}

func TestSafeUserErr_ConnectionRefused(t *testing.T) {
	got := SafeUserErr(errors.New("dial tcp 10.0.0.1:3306: connection refused"))
	if !strings.Contains(got, "连接异常") {
		t.Fatalf("unexpected connection message: %s", got)
	}
}

func TestSafeUserErr_GenericError(t *testing.T) {
	got := SafeUserErr(errors.New("some random internal stack trace"))
	if got != "当前查询暂时未能完成，请稍后重试。" {
		t.Fatalf("unexpected generic message: %s", got)
	}
}

func TestSafeUserErr_RedisConnError(t *testing.T) {
	got := SafeUserErr(errors.New("redis: connection refused"))
	if !strings.Contains(got, "Redis") {
		t.Fatalf("unexpected redis message: %s", got)
	}
}

func TestSafeUserErr_RedisNil(t *testing.T) {
	got := SafeUserErr(errors.New("redis nil result"))
	if !strings.Contains(got, "Redis") {
		t.Fatalf("unexpected redis nil message: %s", got)
	}
}

func TestSafeUserErr_AccessDenied(t *testing.T) {
	got := SafeUserErr(errors.New("access denied for user"))
	if !strings.Contains(got, "权限不足") {
		t.Fatalf("unexpected access denied message: %s", got)
	}
}

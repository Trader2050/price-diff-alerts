package fetcher

import (
	"context"
	"testing"
)

func TestOfficialMissingConfig(t *testing.T) {
	off := NewOfficial(OfficialOptions{}, noopLogger())
	if _, _, err := off.FetchOfficial(context.Background()); err == nil {
		t.Fatal("未配置 RPC 时应报错")
	}

	off = NewOfficial(OfficialOptions{RPCURL: "http://localhost"}, noopLogger())
	if _, _, err := off.FetchOfficial(context.Background()); err == nil {
		t.Fatal("缺少合约地址应报错")
	}
}

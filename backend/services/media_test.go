package services

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
)

func TestVolcPlayInfoQueryUsesV2SignedQueryVerbatim(t *testing.T) {
	inner := "Action=GetPlayInfo&Version=2020-08-01&Vid=video-1&Ssl=1&X-SignedQueries=Action%3BVersion%3BVid%3BSsl&X-Signature=signed"
	token := base64.StdEncoding.EncodeToString([]byte(`{"TokenVersion":"V2","GetPlayInfoToken":"` + inner + `"}`))

	got, err := volcPlayInfoQuery(VolcFormat{VolcId: "video-1", VolcPlayAuthToken: token})
	if err != nil {
		t.Fatalf("volcPlayInfoQuery() error = %v", err)
	}
	if got != inner {
		t.Fatalf("volcPlayInfoQuery() = %q, want exact signed query %q", got, inner)
	}
}

func TestVolcPlayInfoQueryRejectsMismatchedVid(t *testing.T) {
	inner := "Action=GetPlayInfo&Version=2020-08-01&Vid=other-video&X-Signature=signed"
	token := base64.StdEncoding.EncodeToString([]byte(`{"TokenVersion":"V2","GetPlayInfoToken":"` + inner + `"}`))

	_, err := volcPlayInfoQuery(VolcFormat{VolcId: "video-1", VolcPlayAuthToken: token})
	if err == nil || !strings.Contains(err.Error(), "Vid") {
		t.Fatalf("volcPlayInfoQuery() error = %v, want Vid mismatch", err)
	}
}

func TestVolcPrivateDrmQueryAppendsUnsignedRuntimeValues(t *testing.T) {
	signed := "Action=GetPrivateDrmPlayAuth&Version=2020-08-01&Vid=video-1&X-SignedQueries=Action%3BVersion%3BVid&X-Signature=signed"

	got, err := volcPrivateDrmQuery(signed, "auth-a,auth-b", "video-1", "union value")
	if err != nil {
		t.Fatalf("volcPrivateDrmQuery() error = %v", err)
	}
	if !strings.HasPrefix(got, signed+"&") {
		t.Fatalf("volcPrivateDrmQuery() changed signed prefix: %q", got)
	}
	values, err := url.ParseQuery(got)
	if err != nil {
		t.Fatalf("ParseQuery() error = %v", err)
	}
	if values.Get("DrmType") != "webdevice" || values.Get("PlayAuthIds") != "auth-a,auth-b" || values.Get("UnionInfo") != "union value" {
		t.Fatalf("runtime values = %#v", values)
	}
}

func TestVolcPrivateDrmQueryRejectsWrongAction(t *testing.T) {
	_, err := volcPrivateDrmQuery("Action=DeleteSpace&Version=2020-08-01&Vid=video-1", "auth", "video-1", "union")
	if err == nil {
		t.Fatal("volcPrivateDrmQuery() error = nil, want rejected action")
	}
}

func TestDecodeVolcJSONReadsRawAPIResponse(t *testing.T) {
	input := `{"ResponseMetadata":{"RequestId":"request-1"},"Result":{"Vid":"video-1","Status":10}}`
	var response VodPlayInfoResp
	if err := decodeVolcJSON(strings.NewReader(input), &response); err != nil {
		t.Fatalf("decodeVolcJSON() error = %v", err)
	}
	if response.Result.Vid != "video-1" || response.Result.Status != 10 {
		t.Fatalf("decodeVolcJSON() response = %#v", response)
	}
}

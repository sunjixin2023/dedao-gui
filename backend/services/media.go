package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"
)

// MediaBaseInfo media info
type MediaBaseInfo struct {
	MediaType     int    `json:"media_type"` // 1-audio,2-video
	SourceID      string `json:"source_id"`
	SecurityToken string `json:"security_token"`
}

// Audio audio
type Audio struct {
	AliasID        string  `json:"alias_id"`
	Extrainfo      string  `json:"extrainfo"`
	ClassID        int     `json:"class_id"`
	Title          string  `json:"title"`
	ShareTitle     string  `json:"share_title"`
	Mp3PlayURL     string  `json:"mp3_play_url"`
	Duration       int     `json:"duration"`
	Schedule       int     `json:"schedule"`
	TopicID        int     `json:"topic_id"`
	Summary        string  `json:"summary"`
	Price          int     `json:"price"`
	Icon           string  `json:"icon"`
	Size           int     `json:"size"`
	Etag           string  `json:"etag"`
	Token          string  `json:"token"`
	ShareSummary   string  `json:"share_summary"`
	Collection     int     `json:"collection"`
	Count          int     `json:"count"`
	BoredCount     int     `json:"bored_count"`
	AudioType      int     `json:"audio_type"`
	DrmVersion     int     `json:"drm_version"`
	SourceID       int     `json:"source_id"`
	SourceType     int     `json:"source_type"`
	SourceIcon     string  `json:"source_icon"`
	SourceName     string  `json:"source_name"`
	ListenProgress float64 `json:"listen_progress"`
	ListenFinished bool    `json:"listen_finished"`
	DdArticleID    string  `json:"dd_article_id"`
	AudioListIcon  string  `json:"audio_list_icon"`
	ClassCourseID  string  `json:"class_course_id"`
	ClassArticleID string  `json:"class_article_id"`
	LogID          string  `json:"log_id"`
	LogType        string  `json:"log_type"`
	LogInterface   string  `json:"log_interface"`
	Trackinfo      string  `json:"trackinfo"`
	UsedDrm        int     `json:"used_drm"`
	IndexImg       string  `json:"index_img"`
	Reader         string  `json:"reader"`
	ReaderName     string  `json:"reader_name"`
	OdobGroupEnid  string  `json:"odob_group_enid"`
}

type Video struct {
	Token            string  `json:"token"`
	TokenVersion     int     `json:"token_version"`
	CoverImg         string  `json:"cover_img"`
	DdMediaID        int64   `json:"dd_media_id"`
	DdMediaIDStr     string  `json:"dd_media_id_str"`
	Duration         int     `json:"duration"`
	Bitrate480       string  `json:"bitrate_480"`
	Bitrate480Size   int     `json:"bitrate_480_size"`
	Bitrate480Audio  string  `json:"bitrate_480_audio"`
	Bitrate720       string  `json:"bitrate_720"`
	Bitrate720Size   int     `json:"bitrate_720_size"`
	Bitrate720Audio  string  `json:"bitrate_720_audio"`
	Bitrate1080      string  `json:"bitrate_1080"`
	Bitrate1080Size  int     `json:"bitrate_1080_size"`
	Bitrate1080Audio string  `json:"bitrate_1080_audio"`
	IsDrm            bool    `json:"is_drm"`
	ListenProgress   float64 `json:"listen_progress"`
	ListenFinished   bool    `json:"listen_finished"`
	LogID            string  `json:"log_id"`
	LogType          string  `json:"log_type"`
	Caption          string  `json:"caption"`
}

// AudioList audio basic info list
type AudioList struct {
	List []Audio `json:"list"`
}

type MediaVolc struct {
	MediaAliasId string       `json:"media_alias_id"`
	LastModify   time.Time    `json:"last_modify"`
	VersionId    int          `json:"version_id"`
	Tracks       []VideoTrack `json:"tracks"`
}

type VideoTrack struct {
	TrackId      int          `json:"track_id"`
	TrackType    int          `json:"track_type"`
	TrackTypeTag string       `json:"track_type_tag"`
	Duration     int          `json:"duration"`
	Formats      []VolcFormat `json:"formats"`
}

type VolcFormat struct {
	Type              string `json:"type"`
	Format            string `json:"format"`
	VolcId            string `json:"volc_id"`
	VolcPlayAuthToken string `json:"volc_play_auth_token"`
	VolcKeyToken      string `json:"volc_key_token"`
}

type volcPlayAuthEnvelope struct {
	TokenVersion     string `json:"TokenVersion"`
	GetPlayInfoToken string `json:"GetPlayInfoToken"`
}

// VodPlayInfoResp 获取播放地址
// https://www.volcengine.com/docs/4/2918#vodplayinfomodel
type VodPlayInfoResp struct {
	ResponseMetadata VodRespMetadata  `json:"ResponseMetadata"`
	Result           VodPlayInfoModel `json:"Result"`
}

type VodRespMetadata struct {
	RequestId string `json:"RequestId"`
	Action    string `json:"Action"`
	Version   string `json:"Version"`
	Service   string `json:"Service"`
	Region    string `json:"Region"`
}

type VodPlayInfoModel struct {
	Version          int             `json:"Version"`
	Vid              string          `json:"Vid"`
	Status           int             `json:"Status"`
	PosterUrl        string          `json:"PosterUrl"`
	Duration         float64         `json:"Duration"`
	FileType         string          `json:"FileType"`
	EnableAdaptive   bool            `json:"EnableAdaptive"`
	TotalCount       int             `json:"TotalCount"`
	AdaptiveInfo     VodAdaptiveInfo `json:"AdaptiveInfo"`
	PlayInfoList     []VodPlayInfo   `json:"PlayInfoList"`
	ThumbInfoList    []VodThumbInfo  `json:"ThumbInfoList"`
	BarrageMaskUrl   string          `json:"BarrageMaskUrl"`
	SubtitleInfoList []interface{}   `json:"SubtitleInfoList"`
}

type VodAdaptiveInfo struct {
	MainPlayUrl   string `json:"MainPlayUrl"`
	BackupPlayUrl string `json:"BackupPlayUrl"`
	AdaptiveType  string `json:"AdaptiveType"`
}

type VodPlayInfo struct {
	FileId            string      `json:"FileId"`
	Md5               string      `json:"Md5"`
	FileType          string      `json:"FileType"`
	Format            string      `json:"Format"`
	Codec             string      `json:"Codec"`
	Definition        string      `json:"Definition"`
	MainPlayUrl       string      `json:"MainPlayUrl"`
	BackupPlayUrl     string      `json:"BackupPlayUrl"`
	Bitrate           int         `json:"Bitrate"`
	Width             int         `json:"Width"`
	Height            int         `json:"Height"`
	Size              int         `json:"Size"`
	CheckInfo         string      `json:"CheckInfo"`
	IndexRange        string      `json:"IndexRange"`
	InitRange         string      `json:"InitRange"`
	PlayAuth          string      `json:"PlayAuth"`
	PlayAuthId        string      `json:"PlayAuthId"`
	LogoType          string      `json:"LogoType"`
	Quality           string      `json:"Quality"`
	BarrageMaskOffset string      `json:"BarrageMaskOffset"`
	Duration          float64     `json:"Duration"`
	KeyFrameAlignment string      `json:"KeyFrameAlignment"`
	Volume            interface{} `json:"Volume"`
}

type VodThumbInfo struct {
	CaptureNum int      `json:"CaptureNum"`
	StoreUrls  []string `json:"StoreUrls"`
	CellWidth  int      `json:"CellWidth"`
	CellHeight int      `json:"CellHeight"`
	ImgXLen    int      `json:"ImgXLen"`
	ImgYLen    int      `json:"ImgYLen"`
	Interval   float64  `json:"Interval"`
	Format     string   `json:"Format"`
}

type AudioDetail struct {
	Detail Audio `json:"audio_detail"`
}

// AudioByAlias get article audio info
func (s *Service) AudioByAlias(ID string) (list *AudioList, err error) {
	body, err := s.reqAudioByAlias(ID)
	if err != nil {
		return
	}
	defer body.Close()
	if err = handleJSONParse(body, &list); err != nil {
		return
	}
	return
}

// AudioDetailAlias get odob audio info
func (s *Service) AudioDetailAlias(ID string) (detail *Audio, err error) {
	adetail := AudioDetail{}
	body, err := s.reqOdobAudioDetail(ID)
	if err != nil {
		return
	}
	defer body.Close()
	if err = handleJSONParse(body, &adetail); err != nil {
		return
	}
	detail = &adetail.Detail
	return
}

// GetVolcPlayAuthToken  get 火山引擎点播 Vid+PlayAuthToken
func (s *Service) GetVolcPlayAuthToken(mediaIDStr, securityToken string) (info *MediaVolc, err error) {
	body, err := s.reqVolc(mediaIDStr, securityToken)
	if err != nil {
		return
	}
	defer body.Close()
	if err = handleJSONParse(body, &info); err != nil {
		return
	}
	return
}

// GetVolcPlayInfo  火山引擎点播
func (s *Service) GetVolcPlayInfo(query string) (info *VodPlayInfoResp, err error) {
	query, err = normalizeVolcActionQuery(query, "GetPlayInfo")
	if err != nil {
		return nil, err
	}
	body, err := s.reqVolcGetPlayInfo(query)
	if err != nil {
		return
	}
	defer body.Close()
	if err = decodeVolcJSON(body, &info); err != nil {
		return
	}
	return
}

func (s *Service) GetVolcPlayInfoByFormat(format VolcFormat) (*VodPlayInfoResp, error) {
	query, err := volcPlayInfoQuery(format)
	if err != nil {
		return nil, err
	}
	return s.GetVolcPlayInfo(query)
}

func (s *Service) GetVolcPrivateDrmAuthToken(keyToken, playAuthIDs, vid, unionInfo string) (string, error) {
	return volcPrivateDrmQuery(keyToken, playAuthIDs, vid, unionInfo)
}

func decodeVolcJSON(reader io.Reader, target interface{}) error {
	if err := json.NewDecoder(reader).Decode(target); err != nil {
		return fmt.Errorf("解析火山点播响应: %w", err)
	}
	return nil
}

func volcPlayInfoQuery(format VolcFormat) (string, error) {
	vid := strings.TrimSpace(format.VolcId)
	token := strings.TrimSpace(format.VolcPlayAuthToken)
	if vid == "" || token == "" {
		return "", errors.New("火山点播 Vid 或 playAuthToken 为空")
	}

	decoded, decodeErr := base64.StdEncoding.DecodeString(token)
	if decodeErr == nil {
		var envelope volcPlayAuthEnvelope
		if json.Unmarshal(decoded, &envelope) == nil && strings.EqualFold(envelope.TokenVersion, "V2") {
			query := strings.TrimSpace(envelope.GetPlayInfoToken)
			if query == "" {
				return "", errors.New("V2 playAuthToken 缺少 GetPlayInfoToken")
			}
			values, err := validateVolcActionQuery(query, "GetPlayInfo")
			if err != nil {
				return "", err
			}
			if values.Get("Vid") != vid {
				return "", fmt.Errorf("V2 playAuthToken Vid 与媒体 Vid 不一致")
			}
			return query, nil
		}
	}

	return url.Values{
		"Vid":           {vid},
		"PlayAuthToken": {token},
		"Ssl":           {"1"},
	}.Encode(), nil
}

func volcPrivateDrmQuery(keyToken, playAuthIDs, vid, unionInfo string) (string, error) {
	query := strings.TrimSpace(keyToken)
	playAuthIDs = strings.TrimSpace(playAuthIDs)
	vid = strings.TrimSpace(vid)
	unionInfo = strings.TrimSpace(unionInfo)
	if query == "" || playAuthIDs == "" || vid == "" || unionInfo == "" {
		return "", errors.New("私有加密播放参数不完整")
	}
	values, err := validateVolcActionQuery(query, "GetPrivateDrmPlayAuth")
	if err != nil {
		return "", err
	}
	if values.Get("Vid") != vid {
		return "", errors.New("私有加密凭证 Vid 与播放器 Vid 不一致")
	}
	runtimeValues := url.Values{
		"DrmType":     {"webdevice"},
		"PlayAuthIds": {playAuthIDs},
		"UnionInfo":   {unionInfo},
	}
	return query + "&" + runtimeValues.Encode(), nil
}

func normalizeVolcActionQuery(query, expectedAction string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("火山点播请求参数为空")
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return "", fmt.Errorf("解析火山点播请求参数: %w", err)
	}
	if values.Get("Action") == "" {
		return "Action=" + url.QueryEscape(expectedAction) + "&Version=2020-08-01&" + query, nil
	}
	if _, err := validateVolcActionQuery(query, expectedAction); err != nil {
		return "", err
	}
	return query, nil
}

func validateVolcActionQuery(query, expectedAction string) (url.Values, error) {
	if strings.ContainsAny(query, "\r\n") {
		return nil, errors.New("火山点播请求参数包含非法换行")
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return nil, fmt.Errorf("解析火山点播请求参数: %w", err)
	}
	if values.Get("Action") != expectedAction {
		return nil, fmt.Errorf("火山点播 Action 必须为 %s", expectedAction)
	}
	if values.Get("Version") != "2020-08-01" {
		return nil, errors.New("火山点播 Version 必须为 2020-08-01")
	}
	return values, nil
}

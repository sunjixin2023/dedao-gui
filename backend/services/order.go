package services

import (
	"fmt"
	"strings"
)

// OfficialOrderRecord is a normalized purchased-content entry sourced from
// the official study list. It is not a real transaction ledger row.
type OfficialOrderRecord struct {
	RecordID      string `json:"record_id"`
	Title         string `json:"title"`
	Intro         string `json:"intro"`
	Cover         string `json:"cover"`
	Author        string `json:"author"`
	ProductKind   string `json:"product_kind"`
	KindLabel     string `json:"kind_label"`
	ProductType   int    `json:"product_type"`
	ProductID     string `json:"product_id"`
	LearnTargetID string `json:"learn_target_id"`
	PriceText     string `json:"price_text"`
	Progress      int    `json:"progress"`
	ProgressText  string `json:"progress_text"`
	OfficialURL   string `json:"official_url"`
	SourceLabel   string `json:"source_label"`
	UpdatedAt     int    `json:"updated_at"`
}

// OfficialOrderList is a paginated purchased-content snapshot aggregated from
// the official study list.
type OfficialOrderList struct {
	List   []OfficialOrderRecord `json:"list"`
	Total  int                   `json:"total"`
	Page   int                   `json:"page"`
	Limit  int                   `json:"limit"`
	Source string                `json:"source"`
}

// OfficialOrderList returns a normalized purchased-content list that can back
// the order center before real trade APIs are connected.
func (s *Service) OfficialOrderList(page, limit int) (list *OfficialOrderList, err error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 18
	}

	resp, err := s.CourseList("all", "study", "all", page, limit)
	if err != nil {
		return nil, err
	}

	list = &OfficialOrderList{
		List:   []OfficialOrderRecord{},
		Total:  0,
		Page:   page,
		Limit:  limit,
		Source: "study_course_list",
	}
	if resp == nil {
		return list, nil
	}

	list.Total = resp.Total
	for _, item := range resp.List {
		list.List = append(list.List, mapOfficialOrderRecord(item))
	}
	return list, nil
}

func mapOfficialOrderRecord(item Course) OfficialOrderRecord {
	productKind := officialOrderKind(item.Type)
	productID := officialOrderProductID(item, productKind)
	learnTargetID := officialOrderLearnTargetID(item, productKind, productID)

	recordID := fmt.Sprintf("%s:%s", productKind, productID)
	if productID == "" {
		recordID = fmt.Sprintf("%s:%s", productKind, learnTargetID)
	}

	return OfficialOrderRecord{
		RecordID:      recordID,
		Title:         strings.TrimSpace(item.Title),
		Intro:         strings.TrimSpace(firstOrderNonEmpty(item.Intro, item.ProductIntro)),
		Cover:         strings.TrimSpace(item.Icon),
		Author:        strings.TrimSpace(item.Author),
		ProductKind:   productKind,
		KindLabel:     officialOrderKindLabel(productKind, item.Type),
		ProductType:   item.Type,
		ProductID:     productID,
		LearnTargetID: learnTargetID,
		PriceText:     officialOrderPriceText(item),
		Progress:      item.Progress,
		ProgressText:  officialOrderProgressText(item),
		OfficialURL:   strings.TrimSpace(firstOrderNonEmpty(item.DdURL, item.DdExtURL)),
		SourceLabel:   "官方已购内容",
		UpdatedAt:     officialOrderUpdatedAt(item),
	}
}

func officialOrderKind(productType int) string {
	switch productType {
	case 2:
		return "ebook"
	case 13, 1013:
		return "odob"
	case 131:
		return "compass"
	case 310:
		return "trainingcamp"
	case 510:
		return "institute"
	case 4, 22, 36, 65, 66, 67:
		return "course"
	default:
		return "content"
	}
}

func officialOrderKindLabel(kind string, productType int) string {
	switch kind {
	case "ebook":
		return "电子书"
	case "odob":
		return "听书"
	case "compass":
		return "锦囊"
	case "trainingcamp":
		return "训练营"
	case "institute":
		return "研修班"
	case "course":
		if productType == 65 || productType == 67 {
			return "文稿"
		}
		return "课程"
	default:
		return "内容"
	}
}

func officialOrderProductID(item Course, productKind string) string {
	if enid := strings.TrimSpace(item.Enid); enid != "" {
		return enid
	}

	switch productKind {
	case "course":
		return positiveIntString(item.ClassID)
	default:
		return positiveIntString(item.ID)
	}
}

func officialOrderLearnTargetID(item Course, productKind, productID string) string {
	if productKind != "course" {
		return productID
	}
	if id := positiveIntString(item.ID); id != "" {
		return id
	}
	return positiveIntString(item.ClassID)
}

func officialOrderPriceText(item Course) string {
	price := strings.TrimSpace(item.Price)
	switch {
	case price == "":
		if item.ProductPrice > 0 {
			return fmt.Sprintf("¥%.2f", float64(item.ProductPrice)/100)
		}
		return "已购内容"
	case isZeroPriceText(price):
		return "已购内容"
	case strings.HasPrefix(price, "¥"), strings.HasPrefix(price, "￥"), strings.Contains(price, "免费"), strings.Contains(price, "会员"):
		return price
	default:
		return "¥" + price
	}
}

func officialOrderProgressText(item Course) string {
	switch {
	case item.Progress > 0:
		return fmt.Sprintf("进度 %d%%", item.Progress)
	case strings.TrimSpace(item.LastReadInfo) != "":
		return strings.TrimSpace(item.LastReadInfo)
	case strings.TrimSpace(item.LastRead) != "":
		return strings.TrimSpace(item.LastRead)
	default:
		return "已购内容"
	}
}

func officialOrderUpdatedAt(item Course) int {
	if item.LastActionTime > 0 {
		return item.LastActionTime
	}
	return item.CreateTime
}

func positiveIntString(value int) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func isZeroPriceText(value string) bool {
	switch strings.TrimSpace(value) {
	case "0", "0.0", "0.00", "0.000":
		return true
	default:
		return false
	}
}

func firstOrderNonEmpty(values ...string) string {
	for _, item := range values {
		if strings.TrimSpace(item) != "" {
			return item
		}
	}
	return ""
}

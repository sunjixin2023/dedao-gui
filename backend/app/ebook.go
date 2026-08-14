package app

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yann0917/dedao-gui/backend/services"
	"github.com/yann0917/dedao-gui/backend/utils"
	"golang.org/x/sync/errgroup"
)

func EbookDetail(enID string) (detail *services.EbookDetail, err error) {
	detail, err = getService().EbookDetail(enID)
	return
}

// EbookCommentList get ebook 评分&书评
// sort like_count
func EbookCommentList(enID, sort string, page, limit int) (list *services.EbookCommentList, err error) {
	list, err = getService().EbookCommentList(enID, sort, page, limit)
	return
}

// EbookShelfAdd 加入书架
func EbookShelfAdd(enIDs []string) (resp *services.EbookShelfAddResp, err error) {
	resp, err = getService().EbookShelfAdd(enIDs)
	return
}

// EbookShelfRemove 移出书架
func EbookShelfRemove(enIDs []string) (resp *services.EbookShelfAddResp, err error) {
	resp, err = getService().EbookShelfRemove(enIDs)
	return
}

// EbookSyncedNotes 获取官方电子书笔记列表
func EbookSyncedNotes(enID string) (list *services.EbookNoteListResp, err error) {
	detail, err := getService().EbookDetail(enID)
	if err != nil {
		return nil, err
	}
	bookID := 0
	if detail != nil {
		bookID = detail.ID
	}
	list, err = getService().EbookNoteList(enID, bookID, 0)
	return
}

// EbookSyncSave 创建/更新官方电子书笔记
func EbookSyncSave(enID, chapterID, noteLine, note, noteIDHazy string) (resp *services.EbookNoteSaveResp, err error) {
	detail, err := getService().EbookDetail(enID)
	if err != nil {
		return nil, err
	}
	bookID := 0
	if detail != nil {
		bookID = detail.ID
	}

	req := &services.EbookNoteWriteReq{
		BookEnid:         enID,
		BookID:           bookID,
		BookIsOldVersion: 0,
		BookOffset:       0,
		BookSection:      chapterID,
		BookStartPos:     0,
		Location:         chapterID,
		Note:             note,
		NoteLine:         noteLine,
		NoteType:         4,
		RefID:            chapterID,
		State:            5,
		Tags:             []string{},
		NoteIDHazy:       noteIDHazy,
	}

	if noteIDHazy != "" {
		resp, err = getService().EbookNoteUpdate(req)
	} else {
		resp, err = getService().EbookNoteCreate(req)
	}
	return
}

// EbookSyncDelete 删除官方电子书笔记
func EbookSyncDelete(noteIDHazy string) (resp *services.NoteDestroyResp, err error) {
	resp, err = getService().NoteDestroy(noteIDHazy)
	return
}

func EbookInfo(enID string) (info *services.EbookInfo, err error) {
	token, err1 := getService().EbookReadToken(enID)
	if err1 != nil {
		err = err1
		return
	}
	if token == nil {
		err = fmt.Errorf("failed to get ebook read token: token is nil")
		return
	}

	info, err = getService().EbookInfo(token.Token)
	return
}

func EbookReadInfo(enID string) (info *services.EbookInfo, err error) {
	info, err = EbookInfo(enID)
	return
}

func EbookChapterPages(enID, chapterID string) (pages []string, err error) {
	chID := strings.TrimSpace(chapterID)
	if chID == "" {
		err = fmt.Errorf("chapterID is required")
		return
	}

	token, err1 := getService().EbookReadToken(enID)
	if err1 != nil {
		err = err1
		return
	}
	if token == nil {
		err = fmt.Errorf("failed to get ebook read token: token is nil")
		return
	}

	pages, err = generateEbookPages(context.Background(), enID, chID, token.Token, 0, 20, 0)
	return
}

func EbookChapterHtml(enID, chapterID string) (htmlContent string, err error) {
	pages, err1 := EbookChapterPages(enID, chapterID)
	if err1 != nil {
		err = err1
		return
	}
	htmlContent, err = utils.ChapterSvgToHtml(chapterID, pages)
	return
}

func EbookPage(ctx context.Context, enID string) (info *services.EbookInfo, svgContent utils.SvgContents, err error) {
	token, err1 := getService().EbookReadToken(enID)
	if err1 != nil {
		err = err1
		return
	}
	if token == nil {
		err = fmt.Errorf("failed to get ebook read token: token is nil")
		return
	}

	info, err = getService().EbookInfo(token.Token)
	if err != nil {
		return
	}
	total := len(info.BookInfo.Orders)
	chapterLabels := make(map[string]string, len(info.BookInfo.Toc))
	for _, ebookToc := range info.BookInfo.Toc {
		key := ebookToc.Href
		href := strings.Split(ebookToc.Href, "#")
		if len(href) > 1 {
			key = href[0]
		}
		chapterLabels[key] = ebookToc.Text
	}
	coordinator := newChapterProgressCoordinator(total, func(progress Progress) {
		runtime.EventsEmit(ctx, "ebookDownload", progress)
	})
	svgContent, err = fetchEbookChapters(ctx, info.BookInfo.Orders, 5, func(chapterCtx context.Context, order services.EbookOrders, index int) (*utils.SvgContent, error) {
		svgList, err := generateEbookPages(chapterCtx, enID, order.ChapterID, token.Token, 0, 20, 0)
		if err != nil {
			coordinator.recordFailure(err)
			return nil, err
		}

		if err := coordinator.emitSuccess(chapterCtx, chapterLabels[order.ChapterID]); err != nil {
			return nil, err
		}

		return &utils.SvgContent{
			Contents:   svgList,
			ChapterID:  order.ChapterID,
			PathInEpub: order.PathInEpub,
			OrderIndex: index,
		}, nil
	})
	return
}

type chapterFetcher func(context.Context, services.EbookOrders, int) (*utils.SvgContent, error)

type chapterProgressCoordinator struct {
	total          int
	emit           func(Progress)
	mu             sync.Mutex
	completed      atomic.Int64
	failure        error
	beforeEmitHook func()
}

func newChapterProgressCoordinator(total int, emit func(Progress)) *chapterProgressCoordinator {
	return &chapterProgressCoordinator{
		total: total,
		emit:  emit,
	}
}

func (c *chapterProgressCoordinator) recordFailure(err error) {
	if c == nil || err == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failure == nil {
		c.failure = err
	}
}

func (c *chapterProgressCoordinator) emitSuccess(ctx context.Context, label string) error {
	if c == nil {
		if ctx != nil {
			return ctx.Err()
		}
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.failure != nil {
		return c.failure
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	current := c.completed.Add(1)
	progress := Progress{
		Total:   c.total,
		Current: int(current),
		Value:   label,
	}
	if c.total > 0 {
		progress.Pct = int(current) * 100 / c.total
	}

	if c.beforeEmitHook != nil {
		hook := c.beforeEmitHook
		c.beforeEmitHook = nil
		hook()
	}
	if err := ctx.Err(); err != nil {
		c.completed.Add(-1)
		return err
	}
	if c.emit != nil {
		c.emit(progress)
	}
	return nil
}

func fetchEbookChapters(ctx context.Context, orders []services.EbookOrders, limit int, fetch chapterFetcher) (utils.SvgContents, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("limit must be greater than zero")
	}
	if fetch == nil {
		return nil, fmt.Errorf("fetch is required")
	}

	results := make(utils.SvgContents, len(orders))
	if len(orders) == 0 {
		return results, nil
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(limit)
	for index, order := range orders {
		index, order := index, order
		group.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}

			result, err := fetch(groupCtx, order, index)
			if err != nil {
				return err
			}
			if result == nil {
				return fmt.Errorf("fetch returned nil result for chapter %s", order.ChapterID)
			}

			results[index] = result
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func generateEbookPages(ctx context.Context, enid, chapterID, token string, index, count, offset int) (svgList []string, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Try to load from cache first
	if cachedPages, found := services.LoadFromCache(enid, chapterID); found {
		fmt.Printf("使用缓存内容：%s\n", chapterID)
		return cachedPages, nil
	}

	fmt.Printf("下载章节 %s\n", chapterID)
	pageList, err := getService().EbookPagesContext(ctx, chapterID, token, index, count, offset)
	if err != nil {
		return
	}

	for _, item := range pageList.Pages {
		desContents := DecryptAES(item.Svg)
		svgList = append(svgList, desContents)
	}

	if !pageList.IsEnd {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		index += count
		count = 20
		fmt.Printf("下载章节 %s 的更多页面 (索引: %d)\n", chapterID, index)
		list, err1 := generateEbookPages(ctx, enid, chapterID, token, index, count, offset)
		if err1 != nil {
			err = err1
			return
		}

		svgList = append(svgList, list...)
	} else {
		fmt.Printf("章节 %s 下载完成 (共 %d 页)\n", chapterID, len(svgList))
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Save to cache
	if err := services.SaveToCache(enid, chapterID, svgList); err != nil {
		fmt.Printf("警告: 无法缓存章节 %s: %v\n", chapterID, err)
	} else {
		fmt.Printf("已缓存章节 %s\n", chapterID)
	}

	return
}

// PKCS7Unpad 实现PKCS7去填充
func PKCS7Unpad(data []byte) []byte {
	length := len(data)
	unpadding := int(data[length-1])
	return data[:(length - unpadding)]
}

// DecryptAES 实现AES - CBC解密
func DecryptAES(contents string) string {
	ciphertext, err := base64.StdEncoding.DecodeString(contents)
	if err != nil {
		fmt.Println("Base64解码错误:", err)
		return ""
	}

	key := []byte("3e4r06tjkpjcevlbslr3d96gdb5ahbmo")
	iv := []byte("6fd89a1b3a7f48fb")

	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}

	blockSize := block.BlockSize()
	mode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	plaintext = PKCS7Unpad(plaintext)
	return string(plaintext)
}

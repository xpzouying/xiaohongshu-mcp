package humanize

import (
	"errors"

	"github.com/playwright-community/playwright-go"
)

// ownerPage 反查元素所属的 Page。playwright-go 的 ElementHandle 不直接暴露
// 所属 Page，这里经 OwnerFrame().Page() 取得。
func ownerPage(elem playwright.ElementHandle) (playwright.Page, error) {
	frame, err := elem.OwnerFrame()
	if err != nil {
		return nil, err
	}
	if frame == nil {
		return nil, errors.New("元素不属于任何 frame")
	}
	return frame.Page(), nil
}

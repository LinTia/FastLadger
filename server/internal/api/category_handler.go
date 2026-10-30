package api

// 分类 HTTP 适配：GET /books/:id/categories（平铺或 ?tree=1 两级树）与增删改。
import (
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

type categoryCreateReq struct {
	Name     string `json:"name"`
	ParentId int64  `json:"parentId"`
	CatType  int8   `json:"catType"`
}

type categoryUpdateReq struct {
	Name     *string `json:"name"`
	ParentId *int64  `json:"parentId"`
	CatType  *int8   `json:"catType"`
}

func (h *ledgerHandler) listCategories(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	if c.Query("tree") == "1" || c.Query("tree") == "true" {
		tree, err := h.categories.Tree(currentUID(c), bookID)
		if err != nil {
			mapSvcErr(c, err)
			return
		}
		ok(c, tree)
		return
	}
	list, err := h.categories.List(currentUID(c), bookID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, list)
}

func (h *ledgerHandler) createCategory(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	var req categoryCreateReq
	if !bindJSON(c, &req) {
		return
	}
	cat, err := h.categories.Create(currentUID(c), bookID, service.CategoryCreateInput{
		Name:     req.Name,
		ParentId: req.ParentId,
		CatType:  req.CatType,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 广播 data_changed（分类是账本共享配置）
	notifyDataChanged(bookID, cat.SyncVersion)
	ok(c, cat)
}

func (h *ledgerHandler) updateCategory(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	categoryID, good := pathID(c, "cid", "分类 ID 非法")
	if !good {
		return
	}
	var req categoryUpdateReq
	if !bindJSON(c, &req) {
		return
	}
	cat, err := h.categories.Update(currentUID(c), bookID, categoryID, service.CategoryUpdateInput{
		Name:     req.Name,
		ParentId: req.ParentId,
		CatType:  req.CatType,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 广播 data_changed（本次写入的 sync_version 即 cat.SyncVersion）
	notifyDataChanged(bookID, cat.SyncVersion)
	ok(c, cat)
}

func (h *ledgerHandler) deleteCategory(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	categoryID, good := pathID(c, "cid", "分类 ID 非法")
	if !good {
		return
	}
	ver, err := h.categories.Delete(currentUID(c), bookID, categoryID)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 已提交 → 广播 data_changed（删除同样是账本数据变更，版本已递增）
	notifyDataChanged(bookID, ver)
	ok(c, gin.H{"id": categoryID})
}

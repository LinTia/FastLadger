package api

// 报表与导出 HTTP 适配（WBS 6.1 / 6.2）。
//
// 与业务 handler 的差异：export 返回的是**文件**而非 {code,msg,data} 信封——
// 错误时仍走 fail()（JSON 信封），成功时直接写字节流 + Content-Disposition。
// 客户端（Web 端下载逻辑）按 Content-Type 区分成功与错误响应。
import (
	"net/http"
	"strconv"
	"time"

	"fastledger/internal/model"
	"fastledger/internal/service"

	"github.com/gin-gonic/gin"
)

func (h *reportHandler) monthly(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	months, good := queryInt(c, "months")
	if !good {
		return
	}
	points, err := h.svc.Monthly(currentUID(c), bookID, months)
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"items": points})
}

func (h *reportHandler) categoryShare(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	month := c.Query("month")
	if month == "" {
		// 缺省 = 当月（CST 口径，与服务端聚合一致）
		month = time.Now().In(service.CSTZone()).Format("2006-01")
	}
	txType, good := queryInt(c, "type")
	if !good {
		return
	}
	rows, err := h.svc.CategoryShare(currentUID(c), bookID, month, int8(txType))
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	ok(c, gin.H{"month": month, "items": rows})
}

// export 导出交易（WBS 6.2）。format=csv|json；fields=逗号分隔白名单键（缺省全列）。
//
// 行数超过上限时服务层截断，经 X-Export-Truncated: 1 如实告知（不静默少给）。
func (h *reportHandler) export(c *gin.Context) {
	bookID, good := bookID(c)
	if !good {
		return
	}
	format := c.Query("format")
	if format != "csv" && format != "json" {
		fail(c, http.StatusBadRequest, "format 仅支持 csv 或 json")
		return
	}
	txType, good := queryInt(c, "type")
	if !good {
		return
	}
	if txType < 0 || txType > int(model.TxTypeTransfer) { // 0=全部，1~3 与交易类型一致
		fail(c, http.StatusBadRequest, "查询参数 type 非法")
		return
	}
	accountID, good := queryInt64(c, "accountId")
	if !good {
		return
	}
	categoryID, good := queryInt64(c, "categoryId")
	if !good {
		return
	}
	minTime, good := queryInt64(c, "minTime")
	if !good {
		return
	}
	maxTime, good := queryInt64(c, "maxTime")
	if !good {
		return
	}
	fields, err := service.ParseFields(c.Query("fields"))
	if err != nil {
		fail(c, http.StatusBadRequest, "fields 参数非法（字段名见文档）")
		return
	}

	rows, truncated, err := h.svc.ExportRows(currentUID(c), bookID, service.ExportQuery{
		TxType:     int8(txType),
		AccountId:  accountID,
		CategoryId: categoryID,
		MinTime:    minTime,
		MaxTime:    maxTime,
	})
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	if truncated {
		c.Header("X-Export-Truncated", "1")
	}
	// 行数随头返回：Web 端据此判空（"没有可导出的数据"），无需解析文件体
	c.Header("X-Export-Count", strconv.Itoa(len(rows)))

	var (
		body  []byte
		ctype string
	)
	if format == "csv" {
		body, err = service.BuildCSV(rows, fields)
		ctype = "text/csv; charset=utf-8"
	} else {
		body, err = service.BuildExportJSON(rows, fields)
		ctype = "application/json; charset=utf-8"
	}
	if err != nil {
		mapSvcErr(c, err)
		return
	}
	// 文件名：ASCII 兜底 + RFC 5987 UTF-8（与原型 FastLedger_导出_日期.后缀 一致）
	stamp := time.Now().In(service.CSTZone()).Format("20060102")
	ascii := "FastLedger_export_" + stamp + "." + format
	c.Header("Content-Disposition",
		`attachment; filename="`+ascii+`"; filename*=UTF-8''FastLedger_%E5%AF%BC%E5%87%BA_`+stamp+"."+format)
	c.Data(http.StatusOK, ctype, body)
}

// reportHandler 报表/导出路由组。
type reportHandler struct {
	svc *service.ReportService
}

package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	invoicecontract "github.com/dujiao-next/internal/modules/invoice/contract"
	"github.com/dujiao-next/internal/modules/invoice/domain"
)

const apiBaseURL = "https://open.feishu.cn"

type Config struct {
	AppID, AppSecret, BaseToken, TableID string
}

type Client struct {
	cfg        Config
	httpClient *http.Client
	baseURL    string
}

func New(cfg Config) *Client {
	return &Client{cfg: cfg, httpClient: &http.Client{Timeout: 12 * time.Second}, baseURL: apiBaseURL}
}

func (c *Client) Enabled() bool {
	return strings.TrimSpace(c.cfg.AppID) != "" && strings.TrimSpace(c.cfg.AppSecret) != "" && strings.TrimSpace(c.cfg.BaseToken) != "" && strings.TrimSpace(c.cfg.TableID) != ""
}

type apiResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *Client) token(ctx context.Context) (string, error) {
	payload, _ := json.Marshal(map[string]string{"app_id": c.cfg.AppID, "app_secret": c.cfg.AppSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var raw struct {
		Code  int    `json:"code"`
		Msg   string `json:"msg"`
		Token string `json:"tenant_access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return "", err
	}
	if resp.StatusCode/100 != 2 || raw.Code != 0 || strings.TrimSpace(raw.Token) == "" {
		return "", fmt.Errorf("feishu auth failed: code=%d", raw.Code)
	}
	return raw.Token, nil
}

func (c *Client) request(ctx context.Context, method, path string, payload any) (apiResponse, error) {
	token, err := c.token(ctx)
	if err != nil {
		return apiResponse{}, err
	}
	var body io.Reader
	if payload != nil {
		encoded, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return apiResponse{}, encodeErr
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return apiResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return apiResponse{}, err
	}
	defer resp.Body.Close()
	var raw apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&raw); err != nil {
		return raw, err
	}
	if resp.StatusCode/100 != 2 || raw.Code != 0 {
		return raw, fmt.Errorf("feishu api failed: code=%d msg=%s", raw.Code, strings.TrimSpace(raw.Msg))
	}
	return raw, nil
}

func (c *Client) recordPath(suffix string) string {
	return "/open-apis/bitable/v1/apps/" + url.PathEscape(c.cfg.BaseToken) + "/tables/" + url.PathEscape(c.cfg.TableID) + "/records" + suffix
}

func (c *Client) UpsertPaidRequest(ctx context.Context, request *domain.Request) (string, error) {
	if request == nil || !c.Enabled() {
		return "", errors.New("feishu invoice sync unavailable")
	}
	if recordID, err := c.findByRequestNo(ctx, request.RequestNo); err != nil {
		return "", err
	} else if recordID != "" {
		return recordID, nil
	}
	fields := map[string]any{
		"申请编号": request.RequestNo, "申请时间": request.CreatedAt.UnixMilli(), "来源站点": request.SourceHost,
		"订单号/充值单号": request.OriginalOrderNo, "原订单金额": request.OriginalAmount.Decimal.InexactFloat64(),
		"发票类型": map[bool]string{true: "专票", false: "普票"}[request.InvoiceType == domain.TypeSpecial],
		"税点":   float64(request.RateBPS) / 10000, "开票补款": request.InvoiceFeeAmount.Decimal.InexactFloat64(),
		"通道手续费": request.PaymentFeeAmount.Decimal.InexactFloat64(), "实际支付": request.PaymentAmount.Decimal.InexactFloat64(),
		"价税合计": request.InvoiceTotalAmount.Decimal.InexactFloat64(), "发票抬头": request.BuyerTitle,
		"统一社会信用代码": request.TaxNumber, "专票资料": specialInfo(request), "发票接收邮箱": request.RecipientEmail,
		"支付流水": request.ProviderRef, "处理状态": "待开票",
	}
	raw, err := c.request(ctx, http.MethodPost, c.recordPath(""), map[string]any{"fields": fields})
	if err != nil {
		return "", err
	}
	var data struct {
		Record struct {
			RecordID string `json:"record_id"`
		} `json:"record"`
	}
	if err := json.Unmarshal(raw.Data, &data); err != nil || data.Record.RecordID == "" {
		return "", errors.New("feishu create record response invalid")
	}
	return data.Record.RecordID, nil
}

func (c *Client) findByRequestNo(ctx context.Context, requestNo string) (string, error) {
	payload := map[string]any{"filter": map[string]any{"conjunction": "and", "conditions": []any{map[string]any{"field_name": "申请编号", "operator": "is", "value": []string{requestNo}}}}}
	raw, err := c.request(ctx, http.MethodPost, c.recordPath("/search?page_size=1"), payload)
	if err != nil {
		return "", err
	}
	var data struct {
		Items []struct {
			RecordID string `json:"record_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw.Data, &data); err != nil {
		return "", err
	}
	if len(data.Items) == 0 {
		return "", nil
	}
	return data.Items[0].RecordID, nil
}

func specialInfo(request *domain.Request) string {
	if request.InvoiceType != domain.TypeSpecial {
		return ""
	}
	return strings.Join([]string{"地址：" + request.CompanyAddress, "电话：" + request.CompanyPhone, "开户行：" + request.BankName, "账号：" + request.BankAccount}, "\n")
}

func (c *Client) ListReadyInvoices(ctx context.Context) ([]invoicecontract.ReadyInvoice, error) {
	payload := map[string]any{"filter": map[string]any{"conjunction": "and", "conditions": []any{map[string]any{"field_name": "处理状态", "operator": "is", "value": []string{"已开票待发送"}}}}}
	raw, err := c.request(ctx, http.MethodPost, c.recordPath("/search?page_size=100"), payload)
	if err != nil {
		return nil, err
	}
	var data struct {
		Items []struct {
			RecordID string                     `json:"record_id"`
			Fields   map[string]json.RawMessage `json:"fields"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw.Data, &data); err != nil {
		return nil, err
	}
	ready := make([]invoicecontract.ReadyInvoice, 0, len(data.Items))
	for _, item := range data.Items {
		var requestNo, invoiceNumber string
		_ = json.Unmarshal(item.Fields["申请编号"], &requestNo)
		_ = json.Unmarshal(item.Fields["发票号码"], &invoiceNumber)
		var attachments []struct {
			FileToken string `json:"file_token"`
			Name      string `json:"name"`
		}
		_ = json.Unmarshal(item.Fields["发票PDF"], &attachments)
		if strings.TrimSpace(requestNo) == "" || len(attachments) == 0 || strings.TrimSpace(attachments[0].FileToken) == "" {
			continue
		}
		var timestamp int64
		_ = json.Unmarshal(item.Fields["开票日期"], &timestamp)
		var invoiceDate *time.Time
		if timestamp > 0 {
			value := time.UnixMilli(timestamp)
			invoiceDate = &value
		}
		ready = append(ready, invoicecontract.ReadyInvoice{RecordID: item.RecordID, RequestNo: requestNo, InvoiceNumber: invoiceNumber, FileToken: attachments[0].FileToken, FileName: attachments[0].Name, InvoiceDate: invoiceDate})
	}
	return ready, nil
}

func (c *Client) DownloadInvoice(ctx context.Context, fileToken string) ([]byte, error) {
	token, err := c.token(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/open-apis/drive/v1/medias/"+url.PathEscape(fileToken)+"/download", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("feishu attachment download failed: status=%d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 12<<20))
}

func (c *Client) UpdateDelivery(ctx context.Context, recordID, status, lastError string, sentAt *time.Time) error {
	fields := map[string]any{"处理状态": status, "失败原因": lastError}
	if sentAt != nil {
		fields["邮件发送时间"] = sentAt.UnixMilli()
	}
	_, err := c.request(ctx, http.MethodPut, c.recordPath("/"+url.PathEscape(recordID)), map[string]any{"fields": fields})
	return err
}

package bnet

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type API struct {
	client      Client
	UserAgent   string
	Application string
}

func NewAPI(apiKey string) *API {
	return (&API{
		client: &defaultClient{
			h:       http.DefaultClient,
			baseURL: "https://www.bungie.net/Platform",
			apiKey:  apiKey,
		},
		UserAgent: "bungie-api-go/" + Version(),
	})
}

// GetClient returns the API client. You probably don't want to use this.
func (a *API) GetClient() Client {
	return a.client
}

// WithBaseURL sets the base URL. The default is "https://www.bungie.net/Platform".
func (a *API) WithBaseURL(url string) *API {
	url = strings.TrimSuffix(url, "/")
	return a.WithInterceptorFunc(func(base Client, ctx context.Context, r ClientRequest, resp any) error {
		r.BaseURL = url
		return base.Do(ctx, r, resp)
	})
}

func (a *API) WithAuthToken(tok string) *API {
	return a.WithInterceptor(func(c Client) Client {
		return addHeaderClient{c, "Authorization", "Bearer " + tok}
	})
}

func (a *API) WithInterceptor(interceptor func(c Client) Client) *API {
	new := *a
	new.client = interceptor(a.client)
	return &new
}

func (a *API) WithInterceptorFunc(f InterceptorFunc) *API {
	return a.WithInterceptor(func(base Client) Client {
		return InterceptorFuncClient{Base: base, F: f}
	})
}

type addHeaderClient struct {
	base      Client
	headerKey string
	headerVal string
}

func (tc addHeaderClient) Do(ctx context.Context, r ClientRequest, resp any) error {
	if r.Headers == nil {
		r.Headers = map[string]string{}
	}
	r.Headers[tc.headerKey] = tc.headerVal
	return tc.base.Do(ctx, r, resp)
}

func Version() string {
	path, err := os.Executable()
	if err != nil {
		return "0.u"
	}
	bi, err := buildinfo.ReadFile(path)
	if err != nil {
		return "0.u"
	}
	for _, dep := range bi.Deps {
		log.Print(dep)
		if strings.Contains(dep.Path, "bungie-api-go") {
			return dep.Version
		}
	}
	return "0.dev"
}

type defaultClient struct {
	h       *http.Client
	apiKey  string
	baseURL string
}

func (c *defaultClient) Do(ctx context.Context, r ClientRequest, resp any) error {
	requestBody, err := json.Marshal(r.Body)
	if err != nil {
		return err
	}
	baseURL := c.baseURL
	if r.BaseURL != "" {
		baseURL = r.BaseURL
	}
	url := baseURL + getPath(r.PathSpec, r.PathParams, r.QueryParams)
	req, err := http.NewRequestWithContext(ctx, r.Method, url, bytes.NewReader(requestBody))
	req.Header.Set("X-Api-Key", c.apiKey)
	if r.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if err != nil {
		return err
	}
	hResp, err := c.h.Do(req)
	if err != nil {
		return err
	}
	defer hResp.Body.Close()

	bodyBytes, err := io.ReadAll(hResp.Body)
	if err != nil {
		return err
	}

	serverResponse, ok := resp.(interface {
		setRaw([]byte)
		asError() error
	})
	if ok {
		serverResponse.setRaw(bodyBytes)
	}

	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		if hResp.StatusCode > 299 {
			return &HTTPError{Code: hResp.StatusCode, Status: hResp.Status, Body: bodyBytes}
		}
		return err
	}
	if ok {
		return serverResponse.asError()
	}
	return nil
}

func getPath(spec string, params map[string]string, queryParams url.Values) string {
	url := spec
	for field, val := range params {
		url = strings.ReplaceAll(url, "{"+field+"}", val)
	}
	if len(queryParams) != 0 {
		url += "?" + queryParams.Encode()
	}
	return url
}

type DefaultClient *http.Client

type ServerResponse[T any] struct {
	Response           T
	ErrorCode          PlatformErrorCodes
	ThrottleSeconds    int32
	ErrorStatus        string
	Message            string
	MessageData        map[string]string
	DetailedErrorTrace string

	raw json.RawMessage
}

func (r ServerResponse[T]) Raw() []byte {
	return r.raw
}

func (r *ServerResponse[T]) setRaw(b []byte) {
	r.raw = b
}

func (r *ServerResponse[T]) asError() error {
	if r.ErrorCode == PlatformErrorCodes_Success {
		return nil
	}
	return &BungieError{
		Code:            r.ErrorCode,
		Status:          r.ErrorStatus,
		ThrottleSeconds: r.ThrottleSeconds,
	}
}

type BungieError struct {
	Code            PlatformErrorCodes
	Status          string
	ThrottleSeconds int32
}

func (err BungieError) Error() string {
	if err.Status == "" || err.Status == err.Code.Enum() {
		return err.Code.Enum()
	}
	return fmt.Sprintf("%s (%s)", err.Code.Enum(), err.Status)
}

func (err BungieError) Unwrap() error {
	return err.Code
}

type HTTPError struct {
	Code   int
	Status string
	Body   []byte
}

func (err *HTTPError) Error() string {
	if err.Status != "" {
		return err.Status
	}
	return fmt.Sprintf("HTTP Error %d", err.Code)
}

type Int64 int64

func (n *Int64) UnmarshalJSON(raw []byte) error {
	unquoted := bytes.ReplaceAll(raw, []byte{'"'}, []byte{})
	var out int64
	err := json.Unmarshal(unquoted, &out)
	*n = Int64(out)
	return err
}

func (n Int64) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%d"`, n)), nil
}

type Nullable[T any] struct{ v *T }

func (n Nullable[T]) IsNull() bool {
	return n.v == nil
}

func (n Nullable[T]) Value() (v T, ok bool) {
	var zero T
	if n.IsNull() {
		return zero, false
	}
	return *n.v, true
}

func (n Nullable[T]) Must() T {
	if n.v == nil {
		var zero T
		return zero
	}
	return *n.v
}

func (n *Nullable[T]) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		n.v = nil
	}

	var val T
	if err := json.Unmarshal(raw, &val); err != nil {
		return err
	}
	n.v = &val
	return nil
}

func (n Nullable[T]) MarshalJSON() ([]byte, error) {
	if n.v == nil {
		return []byte("null"), nil
	}
	return json.Marshal(n.v)
}

func (n Nullable[T]) Format(f fmt.State, verb rune) {
	if n.IsNull() {
		io.WriteString(f, "null")
		return
	}
	fmt.Fprintf(f, fmt.FormatString(f, verb), n.Must())
}

type Timestamp string

//type Hash[T ActivityDefinition | ActivityGraphDefinition | ActivityModeDefinition | ActivityModifierDefinition | ActivityTypeDefinition | ArtifactDefinition | BreakerTypeDefinition | ChecklistDefinition | ClassDefinition | CollectibleDefinition | DamageTypeDefinition | DestinationDefinition | EnergyTypeDefinition | EquipmentSlotDefinition | EventCardDefinition | FactionDefinition | GenderDefinition | GuardianRankConstantsDefinition | GuardianRankDefinition | InventoryBucketDefinition | InventoryItemDefinition | ItemCategoryDefinition | ItemTierTypeDefinition | LoadoutColorDefinition | LoadoutConstantsDefinition | LoadoutIconDefinition | LoadoutNameDefinition | LocationDefinition | LoreDefinition | MaterialRequirementSetDefinition | MedalTierDefinition | MetricDefinition | MilestoneDefinition | ObjectiveDefinition | PlaceDefinition | PlugSetDefinition | PowerCapDefinition | PresentationNodeDefinition | ProgressionDefinition | ProgressionLevelRequirementDefinition | ProgressionMappingDefinition | RaceDefinition | RecordDefinition | ReportReasonCategoryDefinition | RewardSourceDefinition | SandboxPatternDefinition | SandboxPerkDefinition | SeasonDefinition | SeasonPassDefinition | SocialCommendationDefinition | SocialCommendationNodeDefinition | SocketCategoryDefinition | SocketTypeDefinition | StatDefinition | StatGroupDefinition | TalentGridDefinition | TraitDefinition | UnlockDefinition | UnlockValueDefinition | VendorDefinition | VendorGroupDefinition] uint32

type defTable interface {
	DefinitionTable() string
}

type Hash[T defTable] uint32

type BitmaskSet[T ~uint32 | ~uint64 | ~int32 | OptInFlags] uint64

type DefSource interface {
	GetDef(table string, hash uint32, out any) error
}

// func (h Hash[T]) Get(fetcher func(Hash[T]) (*T, error)) (*T, error) {
// 	return fetcher(h)
// }

func (h Hash[T]) Get(defs DefSource) (*T, error) {
	var t T
	if err := defs.GetDef(t.DefinitionTable(), uint32(h), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (a API) GetDef(table string, hash uint32, out any) error {
	ctx := context.Background()
	def, err := a.Destiny2GetDestinyEntityDefinition(ctx, Destiny2GetDestinyEntityDefinitionRequest{
		EntityType:     table,
		HashIdentifier: hash,
	})
	if err != nil {
		return err
	}
	r := struct {
		Response any
	}{}
	r.Response = out
	return json.Unmarshal(def.Raw(), &r)
}

func (b BitmaskSet[T]) Has(value T) bool {
	return uint64(b)&uint64(value) != 0
}

func (b BitmaskSet[T]) HasAll(vals ...T) bool {
	for _, value := range vals {
		if !b.Has(value) {
			return false
		}
	}
	return true
}

func (b BitmaskSet[T]) Add(value T) BitmaskSet[T] {
	return BitmaskSet[T](uint64(b) | uint64(value))
}

func (b BitmaskSet[T]) Remove(value T) BitmaskSet[T] {
	return BitmaskSet[T](uint64(b) & ^uint64(value))
}

func (b BitmaskSet[T]) Clear(value T) BitmaskSet[T] {
	return BitmaskSet[T](0)
}

type BaseItemComponentSet[T comparable] struct {
	Objectives ComponentResponse[map[T]ItemObjectivesComponent] `json:"objectives"`
}

type ItemComponentSet[T comparable] struct {
	Sockets        ComponentResponse[map[T]ItemSocketsComponent]        `json:"sockets"`
	PlugObjectives ComponentResponse[map[T]ItemPlugObjectivesComponent] `json:"plugObjectives"`
	PlugStates     ComponentResponse[map[T]ItemPlugComponent]           `json:"plugStates"`
	Perks          ComponentResponse[map[T]ItemPerksComponent]          `json:"perks"`
	RenderData     ComponentResponse[map[T]ItemRenderComponent]         `json:"renderData"`
	Stats          ComponentResponse[map[T]ItemStatsComponent]          `json:"stats"`
	Objectives     ComponentResponse[map[T]ItemObjectivesComponent]     `json:"objectives"`
	Instances      ComponentResponse[map[T]ItemInstanceComponent]       `json:"instances"`
	ReusablePlugs  ComponentResponse[map[T]ItemReusablePlugsComponent]  `json:"reusablePlugs"`
	TalentGrids    ComponentResponse[map[T]ItemTalentGridComponent]     `json:"talentGrids"`
}

type VendorSaleItemSetComponent[T any] struct {
	SaleItems map[int32]T `json:"saleItems"`
}

func joinArray[T any](a []T) string {
	var out []string
	for _, v := range a {
		out = append(out, fmt.Sprint(v))
	}
	return strings.Join(out, ",")
}

type InterceptorFunc func(base Client, ctx context.Context, r ClientRequest, resp any) error

type InterceptorFuncClient struct {
	Base Client
	F    InterceptorFunc
}

func (fc InterceptorFuncClient) Do(ctx context.Context, r ClientRequest, resp any) error {
	return fc.F(fc.Base, ctx, r, resp)
}

type ClientRequest struct {
	Operation   string
	Method      string
	BaseURL     string
	PathSpec    string
	Headers     map[string]string
	PathParams  map[string]string
	QueryParams url.Values
	Body        any
}

type Client interface {
	Do(ctx context.Context, r ClientRequest, resp any) error
}

func (err PlatformErrorCodes) Error() string {
	return err.Enum()
}

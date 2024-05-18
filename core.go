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
	return &API{
		client: &defaultClient{
			h:      http.DefaultClient,
			base:   "https://www.bungie.net/Platform",
			apiKey: apiKey,
		},
		UserAgent: "bungie-api-go/" + Version(),
	}
}

func (a *API) withInterceptor(interceptor func(c Client) Client) *API {
	new := *a
	new.client = interceptor(a.client)
	return &new
}

func (a *API) WithAuthToken(tok string) *API {
	return a.withInterceptor(func(c Client) Client {
		return addHeaderClient{c, "Authorization", "Bearer " + tok}
	})
}

type addHeaderClient struct {
	base      Client
	headerKey string
	headerVal string
}

func (tc addHeaderClient) Do(ctx context.Context, operation string, method string, pathSpec string, headers map[string]string, pathParams map[string]string, queryParams url.Values, body any, resp any) error {
	if headers == nil {
		headers = map[string]string{}
	}
	headers[tc.headerKey] = tc.headerVal
	return tc.base.Do(ctx, operation, method, pathSpec, headers, pathParams, queryParams, body, resp)
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

type Client interface {
	Do(ctx context.Context,
		operation string,
		method string,
		pathSpec string,
		headers map[string]string,
		pathParams map[string]string,
		queryParams url.Values,
		body any,
		resp any) error
}

type defaultClient struct {
	h      *http.Client
	base   string
	apiKey string
}

func (c *defaultClient) Do(ctx context.Context,
	operation string,
	method string,
	pathSpec string,
	headers map[string]string,
	pathParams map[string]string,
	queryParams url.Values,
	body any,
	resp any) error {

	requestBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := c.base + getPath(pathSpec, pathParams, queryParams)
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(requestBody))
	req.Header.Set("X-Api-Key", c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
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
	if serverResponse, ok := resp.(interface{ setRaw([]byte) }); ok {
		serverResponse.setRaw(bodyBytes)
	}
	return json.Unmarshal(bodyBytes, &resp)
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

type Timestamp string

//type Hash[T ActivityDefinition | ActivityGraphDefinition | ActivityModeDefinition | ActivityModifierDefinition | ActivityTypeDefinition | ArtifactDefinition | BreakerTypeDefinition | ChecklistDefinition | ClassDefinition | CollectibleDefinition | DamageTypeDefinition | DestinationDefinition | EnergyTypeDefinition | EquipmentSlotDefinition | EventCardDefinition | FactionDefinition | GenderDefinition | GuardianRankConstantsDefinition | GuardianRankDefinition | InventoryBucketDefinition | InventoryItemDefinition | ItemCategoryDefinition | ItemTierTypeDefinition | LoadoutColorDefinition | LoadoutConstantsDefinition | LoadoutIconDefinition | LoadoutNameDefinition | LocationDefinition | LoreDefinition | MaterialRequirementSetDefinition | MedalTierDefinition | MetricDefinition | MilestoneDefinition | ObjectiveDefinition | PlaceDefinition | PlugSetDefinition | PowerCapDefinition | PresentationNodeDefinition | ProgressionDefinition | ProgressionLevelRequirementDefinition | ProgressionMappingDefinition | RaceDefinition | RecordDefinition | ReportReasonCategoryDefinition | RewardSourceDefinition | SandboxPatternDefinition | SandboxPerkDefinition | SeasonDefinition | SeasonPassDefinition | SocialCommendationDefinition | SocialCommendationNodeDefinition | SocketCategoryDefinition | SocketTypeDefinition | StatDefinition | StatGroupDefinition | TalentGridDefinition | TraitDefinition | UnlockDefinition | UnlockValueDefinition | VendorDefinition | VendorGroupDefinition] uint32

type defTable interface {
	DefinitionTable() string
}

type Hash[T defTable] uint32

type BitmaskSet[T ~uint32 | ~uint64 | ~int32 | OptInFlags] uint64

func (h Hash[T]) Get(fetcher func(Hash[T]) (*T, error)) (*T, error) {
	return fetcher(h)
}

func (h Hash[T]) Fetch(api *API) (*T, error) {
	var t T
	if err := api.GetDef(t.DefinitionTable(), uint32(h), &t); err != nil {
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

package bnet

import (
	"context"
	"net/http"
)

type API struct {
	client Client
}

type Client interface {
	Get(ctx context.Context, pathSpec string, pathParams map[string]string) ([]byte, error)
	Post(ctx context.Context, pathSpec string, pathParams map[string]string, body []byte) ([]byte, error)
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
}

type Nullable[T any] *T

type Timestamp string

type Hash[T any] uint32

type BitmaskSet[T ~uint32 | ~uint64 | ~int32 | OptInFlags] uint64

func (h Hash[T]) Get(fetcher func(Hash[T]) (*T, error)) (*T, error) {
	return fetcher(h)
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

/*
// DictionaryComponentResponse
type DictionaryComponentResponse[K comparable, V any] struct {
	Data map[K]V `json:"data"`

	Privacy ComponentPrivacySetting `json:"privacy"`

	// If true, this component is disabled.
	Disabled Nullable[bool] `json:"disabled"`
}

type SingleComponentResponse[T any] struct {
	Data T `json:"data"`

	Privacy ComponentPrivacySetting `json:"privacy"`

	// If true, this component is disabled.
	Disabled Nullable[bool] `json:"disabled"`
}
*/

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

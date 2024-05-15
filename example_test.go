package bnet_test

import (
	"context"
	"log"

	bnet "github.com/d2orbc/bungie-api-go"
)

func Example_A() {
	ctx := context.Background()
	defs := &Defs{}
	var api = &bnet.API{}

	resp, err := api.Destiny2GetPostGameCarnageReport(ctx,
		bnet.Destiny2GetPostGameCarnageReportRequest{
			ActivityID: 10_000_000_000,
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	if resp.ErrorCode == bnet.PlatformErrorCodes_DestinyPGCRNotFound {
		log.Fatal("PGCR not found")
	}

	activity, _ := resp.Response.ActivityDetails.DirectorActivityHash.Get(defs.Activity)
	dest, _ := activity.DestinationHash.Get(defs.Destination)
	log.Print(dest.DisplayProperties.Name)

	for _, entry := range resp.Response.Entries {
		for _, weapon := range entry.Extended.Weapons {
			weaponDef, _ := weapon.ReferenceID.Get(defs.InventoryItem)
			weaponName := weaponDef.DisplayProperties.Name
			kills := weapon.Values["uniqueWeaponKills"].Basic.Value
			log.Printf("%s: %d", weaponName, int(kills))
		}
	}
}

func Example_B() {
	ctx := context.Background()
	// defs := &Defs{}
	var api = &bnet.API{}

	resp, err := api.Destiny2GetProfile(ctx, bnet.Destiny2GetProfileRequest{
		Components: []bnet.ComponentType{
			bnet.ComponentType_ProfileProgression,
			bnet.ComponentType_Transitory,
		},
		DestinyMembershipID: 4611686018504534611,
		MembershipType:      bnet.BungieMembershipType_TigerSteam,
	})
	if err != nil {
		log.Fatal(err)
	}

	resp.Response.Profile.Data.VersionsOwned.Has(bnet.GameVersions_BeyondLight)
	for char, items := range resp.Response.CharacterUninstancedItemComponents {
		log.Print(char, items)
	}
}

type Defs struct{}

func (f *Defs) Destination(h bnet.Hash[bnet.DestinationDefinition]) (*bnet.DestinationDefinition, error) {
	return nil, nil
}

func (f *Defs) Activity(h bnet.Hash[bnet.ActivityDefinition]) (*bnet.ActivityDefinition, error) {
	return nil, nil
}

func (f *Defs) InventoryItem(h bnet.Hash[bnet.InventoryItemDefinition]) (*bnet.InventoryItemDefinition, error) {
	return nil, nil
}

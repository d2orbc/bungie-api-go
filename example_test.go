package bnet_test

import (
	"context"
	"log"

	bnet "github.com/d2orbc/bungie-api-go"
)

var defs bnet.DefSource

func Example_A() {
	ctx := context.Background()
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

	activity, _ := resp.Response.ActivityDetails.DirectorActivityHash.Get(defs)
	dest, _ := activity.DestinationHash.Get(defs)
	log.Print(dest.DisplayProperties.Name)

	for _, entry := range resp.Response.Entries {
		for _, weapon := range entry.Extended.Weapons {
			weaponDef, _ := weapon.ReferenceID.Get(defs)
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

func Example_Chiri() {
	ctx := context.Background()
	var api = &bnet.API{}

	const (
		memType = bnet.BungieMembershipType_TigerSteam
		memId   = 4611686018504534611
	)

	profile, err := api.Destiny2GetProfile(ctx, bnet.Destiny2GetProfileRequest{
		MembershipType:      memType,
		DestinyMembershipID: memId,
		Components: []bnet.ComponentType{
			bnet.ComponentType_Characters,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	if eventHash, ok := profile.Response.Profile.Data.ActiveEventCardHash.Value(); ok {
		eventDef, _ := eventHash.Get(defs)
		log.Print(eventDef.DisplayProperties.Name)
	}

	vendorSeen := make(map[uint32]bool)
	for charId := range profile.Response.Characters.Data {
		vendors, err := api.Destiny2GetVendors(ctx, bnet.Destiny2GetVendorsRequest{
			MembershipType:      memType,
			DestinyMembershipID: memId,
			CharacterID:         charId,
			Components: []bnet.ComponentType{
				bnet.ComponentType_Vendors,
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		for vendorHash := range vendors.Response.Vendors.Data {
			if vendorSeen[vendorHash] {
				continue
			}
			vendorSeen[vendorHash] = true
			// NOTE: we have to cast here because Bungie doesn't type the map key.
			vendorDef, _ := bnet.Hash[bnet.VendorDefinition](vendorHash).Get(defs)
			for _, loc := range vendorDef.Locations {
				locationDef, _ := loc.DestinationHash.Get(defs)
				log.Printf("%s %s", vendorDef.DisplayProperties.Name, locationDef.DisplayProperties.Name)
			}
		}
	}
}

func getMissingMemType(memID bnet.Int64) bnet.BungieMembershipType {
	ctx := context.Background()
	var api = &bnet.API{}

	var (
		memType = bnet.BungieMembershipType_All
	)

	resp, _ := api.UserGetMembershipDataById(ctx, bnet.UserGetMembershipDataByIdRequest{
		MembershipID:   memID,
		MembershipType: memType,
	})
	for _, mem := range resp.Response.DestinyMemberships {
		if mem.MembershipID == memID {
			return mem.MembershipType
		}
	}
	return bnet.BungieMembershipType_None
}

func Example_getDeletedCharacterIds_API(memID bnet.Int64, membershipType bnet.BungieMembershipType) {
	ctx := context.Background()
	var api = &bnet.API{}

	resp, _ := api.Destiny2GetHistoricalStatsForAccount(ctx, bnet.Destiny2GetHistoricalStatsForAccountRequest{
		DestinyMembershipID: memID,
		MembershipType:      membershipType,
	})
	var deletedChars []bnet.Int64
	for _, char := range resp.Response.Characters {
		if char.Deleted {
			deletedChars = append(deletedChars, char.CharacterID)
		}
	}
	log.Print(deletedChars)
}

func Example_getClan_API(memID bnet.Int64, membershipType bnet.Nullable[bnet.BungieMembershipType]) {
	ctx := context.Background()
	var api = &bnet.API{}

	memType, ok := membershipType.Value()
	if !ok {
		memType = getMissingMemType(memID)
	}

	resp, _ := api.GroupV2GetGroupsForMember(ctx, bnet.GroupV2GetGroupsForMemberRequest{
		MembershipID:   memID,
		MembershipType: memType,
		Filter:         bnet.GroupsForMemberFilter_All,
		GroupType:      bnet.GroupType_Clan,
	})
	for _, clan := range resp.Response.Results {
		if clan.Group.GroupType == bnet.GroupType_Clan {
			log.Print(clan.Group.Name)
			log.Print(clan.Group.ClanInfo.ClanCallsign)
			log.Print(clan.Group.GroupID)
		}
	}
}

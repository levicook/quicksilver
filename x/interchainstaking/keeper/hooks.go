package keeper

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	epochstypes "github.com/ingenuity-build/quicksilver/x/epochs/types"
	icqtypes "github.com/ingenuity-build/quicksilver/x/interchainquery/types"
	"github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
)

func (k Keeper) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
}

func (k Keeper) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	// every epoch
	k.Logger(ctx).Info("Handling epoch end")
	if epochIdentifier == "epoch" {
		k.IterateRegisteredZones(ctx, func(index int64, zoneInfo types.RegisteredZone) (stop bool) {
			k.Logger(ctx).Info("Taking a snapshot of intents")
			k.AggregateIntents(ctx, zoneInfo)
			if zoneInfo.WithdrawalWaitgroup > 0 {
				k.Logger(ctx).Error("Epoch waitgroup was unexpected > 0; this means we did not process the previous epoch!")
				zoneInfo.WithdrawalWaitgroup = 0
			}
			// OnChanOpenAck calls SetWithdrawalAddress (see ibc_module.go)
			for _, da := range zoneInfo.GetDelegationAccounts() {
				k.Logger(ctx).Info("Withdrawing rewards")

				var rewardscb Callback = func(k Keeper, ctx sdk.Context, args []byte, query icqtypes.Query) error {
					zone, found := k.GetRegisteredZoneInfo(ctx, query.GetChainId())
					if !found {
						return fmt.Errorf("no registered zone for chain id: %s", query.GetChainId())
					}

					return k.WithdrawDelegationRewardsForResponse(ctx, zone, query.QueryParameters["delegator"], args)
				}

				var delegationcb Callback = func(k Keeper, ctx sdk.Context, args []byte, query icqtypes.Query) error {
					zone, found := k.GetRegisteredZoneInfo(ctx, query.GetChainId())
					if !found {
						return fmt.Errorf("no registered zone for chain id: %s", query.GetChainId())
					}

					return k.UpdateDelegationRecordsForAddress(ctx, zone, query.QueryParameters["address"], args)
				}

				k.ICQKeeper.MakeRequest(
					ctx,
					zoneInfo.ConnectionId,
					zoneInfo.ChainId,
					"cosmos.staking.v1beta1.Query/DelegatorDelegations",
					map[string]string{"address": da.Address},
					sdk.NewInt(-1),
					types.ModuleName,
					delegationcb,
				)

				k.ICQKeeper.MakeRequest(
					ctx,
					zoneInfo.ConnectionId,
					zoneInfo.ChainId,
					"cosmos.distribution.v1beta1.Query/DelegationTotalRewards",
					map[string]string{"delegator": da.Address},
					sdk.NewInt(-1),
					types.ModuleName,
					rewardscb,
				)

				zoneInfo.WithdrawalWaitgroup++
				k.Logger(ctx).Info("Incrementing waitgroup for delegation", "value", zoneInfo.WithdrawalWaitgroup)
			}
			k.SetRegisteredZone(ctx, zoneInfo)

			return false
		})
	}
}

// ___________________________________________________________________________________________________

// Hooks wrapper struct for incentives keeper
type Hooks struct {
	k Keeper
}

var _ epochstypes.EpochHooks = Hooks{}

func (k Keeper) Hooks() Hooks {
	return Hooks{k}
}

// epochs hooks
func (h Hooks) BeforeEpochStart(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	h.k.BeforeEpochStart(ctx, epochIdentifier, epochNumber)
}

func (h Hooks) AfterEpochEnd(ctx sdk.Context, epochIdentifier string, epochNumber int64) {
	h.k.AfterEpochEnd(ctx, epochIdentifier, epochNumber)
}

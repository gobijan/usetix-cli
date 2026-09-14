package commands

import (
	"fmt"
	"io"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func NewPromoCodes(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "promo-codes", Short: "Manage promo codes and promoter assignments"}
	command.AddCommand(newPromoCodesList(runtime), newPromoCodeShow(runtime), newPromoCodeWrite(runtime, true), newPromoCodeWrite(runtime, false), newPromoCodeActivation(runtime, false), newPromoCodeActivation(runtime, true))
	return command
}

func newPromoCodesList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List active and inactive promo codes with their sales links", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListPromoCodes(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, len(response.PromoCodes))
				return err
			}
			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs {
				ids := make([]map[string]any, 0, len(response.PromoCodes))
				for _, code := range response.PromoCodes {
					ids = append(ids, map[string]any{"id": code.ID})
				}
				data = ids
			}
			return runtime.Output().OK(data, func(w io.Writer) error {
				for _, code := range response.PromoCodes {
					if err := renderPromoCode(code)(w); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}

func newPromoCodeShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "show ID", Short: "Show a promo code and its shareable link", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := positiveID(args[0], "promo code ID")
		if err != nil {
			return err
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		code, err := client.GetPromoCode(command.Context(), id)
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(code, renderPromoCode(code))
	}}
}

type promoCodeFlags struct {
	code, discountType, amount, event, expires string
	promoter                                   int64
	limit, perCustomer                         int
}

func newPromoCodeWrite(runtime *appctx.Runtime, create bool) *cobra.Command {
	flags := &promoCodeFlags{}
	command := &cobra.Command{Use: "update ID", Short: "Update a code; omit flags to preserve existing values", Args: cobra.ExactArgs(1)}
	if create {
		command.Use = "create"
		command.Short = "Create a code (defaults to 0% tracking without a discount)"
		command.Args = cobra.NoArgs
	}
	command.RunE = func(command *cobra.Command, args []string) error {
		var id int64
		var err error
		if !create {
			id, err = positiveID(args[0], "promo code ID")
			if err != nil {
				return err
			}
		}
		if flags.promoter < 0 || flags.limit < 0 || flags.perCustomer < 0 {
			return output.ErrUsage("--promoter, --usage-limit and --max-per-customer cannot be negative")
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		attributes, err := flags.attributes(command, client, create)
		if err != nil {
			return err
		}
		if create {
			code, location, err := client.CreatePromoCode(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(code, renderPromoCode(code), output.WithMeta("location", location))
		}
		if len(attributes) == 0 {
			return output.ErrUsage("provide at least one field to update")
		}
		code, err := client.UpdatePromoCode(command.Context(), id, attributes)
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(code, renderPromoCode(code))
	}
	command.Flags().StringVar(&flags.code, "code", "", "code text")
	command.Flags().StringVar(&flags.discountType, "discount-type", "percentage", "percentage or fixed")
	command.Flags().StringVar(&flags.amount, "discount-amount", "0", "discount amount; 0 tracks sales without a discount")
	command.Flags().StringVar(&flags.event, "event", "", "event slug; empty string clears the event on update")
	command.Flags().Int64Var(&flags.promoter, "promoter", 0, "promoter membership ID; 0 clears the assignment on update")
	command.Flags().StringVar(&flags.expires, "expires-at", "", "ISO8601 expiry; empty string clears it")
	command.Flags().IntVar(&flags.limit, "usage-limit", 0, "total redemption cap; 0 clears it")
	command.Flags().IntVar(&flags.perCustomer, "max-per-customer", 0, "redemption cap per buyer; 0 clears it")
	if create {
		_ = command.MarkFlagRequired("code")
	}
	return command
}

func (flags *promoCodeFlags) attributes(command *cobra.Command, client *api.Client, create bool) (map[string]any, error) {
	attrs := map[string]any{}
	for flag, field := range map[string]string{"code": "code", "discount-type": "discount_type", "discount-amount": "discount_amount", "expires-at": "expires_at"} {
		if command.Flags().Changed(flag) || (create && flag != "expires-at") {
			value, _ := command.Flags().GetString(flag)
			attrs[field] = value
		}
	}
	if command.Flags().Changed("promoter") {
		attrs["promoter_membership_id"] = nil
		if flags.promoter > 0 {
			attrs["promoter_membership_id"] = flags.promoter
		}
	}
	if command.Flags().Changed("event") {
		attrs["event_id"] = nil
		if flags.event != "" {
			event, err := client.GetEvent(command.Context(), flags.event)
			if err != nil {
				return nil, NormalizeError(err)
			}
			attrs["event_id"] = event.ID
		}
	}
	for flag, field := range map[string]string{"usage-limit": "usage_limit", "max-per-customer": "max_per_customer"} {
		if command.Flags().Changed(flag) {
			value, _ := command.Flags().GetInt(flag)
			attrs[field] = nil
			if value > 0 {
				attrs[field] = value
			}
		}
	}
	return attrs, nil
}

func newPromoCodeActivation(runtime *appctx.Runtime, active bool) *cobra.Command {
	action := "deactivate"
	if active {
		action = "reactivate"
	}
	var yes bool
	command := &cobra.Command{Use: action + " ID", Short: action + " a promo code without deleting its history", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := positiveID(args[0], "promo code ID")
		if err != nil {
			return err
		}
		if !yes {
			return output.ErrUsage(action + " requires --yes")
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		code, err := client.UpdatePromoCode(command.Context(), id, map[string]any{"active": active})
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(code, renderPromoCode(code))
	}}
	command.Flags().BoolVar(&yes, "yes", false, "confirm changing code availability")
	return command
}

func renderPromoCode(code api.PromoCode) output.StyledRenderer {
	return func(w io.Writer) error {
		state := "inactive"
		if code.Active {
			state = "active"
		}
		promoter := "unassigned"
		if code.PromoterMembershipID != nil {
			promoter = fmt.Sprintf("promoter %d", *code.PromoterMembershipID)
		}
		_, err := fmt.Fprintf(w, "%d  %s  %s %s  %s  %s\n%s\n", code.ID, terminal.SanitizeLine(code.Code), terminal.SanitizeLine(code.DiscountAmount), terminal.SanitizeLine(code.DiscountType), state, promoter, terminal.SanitizeLine(code.ShareURL))
		return err
	}
}

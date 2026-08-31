package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTicketValidation(t *testing.T) {
	tests := []struct {
		name    string
		ticket  Ticket
		wantErr bool
	}{
		{name: "general ticket", ticket: Ticket{Subject: "Login", Category: TicketCategoryGeneral, Priority: TicketPriorityNormal}, wantErr: false},
		{name: "refund ticket", ticket: Ticket{Subject: "Manual refund", Category: TicketCategoryRefund, Priority: TicketPriorityHigh}, wantErr: false},
		{name: "invalid category", ticket: Ticket{Subject: "x", Category: "balance", Priority: TicketPriorityNormal}, wantErr: true},
		{name: "empty subject", ticket: Ticket{Category: TicketCategoryGeneral, Priority: TicketPriorityNormal}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ticket.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTicketMessageValidationRejectsEmptyBodyAndUnknownRole(t *testing.T) {
	require.Error(t, (&TicketMessage{AuthorID: 1, AuthorRole: TicketAuthorRoleUser}).Validate())
	require.Error(t, (&TicketMessage{AuthorID: 1, AuthorRole: "system", Body: "hello"}).Validate())
	require.NoError(t, (&TicketMessage{AuthorID: 1, AuthorRole: TicketAuthorRoleAdmin, Body: "hello"}).Validate())
}

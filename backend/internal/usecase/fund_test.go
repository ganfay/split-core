package usecase

import (
	"testing"

	"github.com/ganfay/split-core/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestSettleUp(t *testing.T) {
	type testCase struct {
		name      string
		purchases []domain.Purchase
		members   []domain.User
		expected  *domain.Settlement
	}

	tests := []testCase{
		{
			name: "One paid for three",
			purchases: []domain.Purchase{
				{
					ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 300, Description: "First payment description",
				},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
			},
			expected: &domain.Settlement{
				TotalAmount: 300,
				Average:     100,
				Debts: []domain.Debt{
					{
						FromID: 2, ToID: 1, Amount: 100,
					},
					{
						FromID: 3, ToID: 1, Amount: 100,
					},
				},
			},
		},
		{
			name: "Equally",
			purchases: []domain.Purchase{
				{
					ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 300, Description: "First payment description",
				},
				{
					ID: 2, FundID: 1, Payer: domain.User{ID: 2, Username: "SecondUser"}, Amount: 300, Description: "Second payment description",
				},
				{
					ID: 3, FundID: 1, Payer: domain.User{ID: 3, Username: "ThirdUser"}, Amount: 300, Description: "Third payment description",
				},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
			},
			expected: &domain.Settlement{
				TotalAmount: 900,
				Average:     300,
				Debts:       []domain.Debt(nil),
			},
		},
		{
			name: "Complex float precision",
			purchases: []domain.Purchase{
				{
					ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 123.52, Description: "First payment description",
				},
				{
					ID: 2, FundID: 1, Payer: domain.User{ID: 2, Username: "SecondUser"}, Amount: 2000, Description: "Second payment description",
				},
				{
					ID: 3, FundID: 1, Payer: domain.User{ID: 3, Username: "ThirdUser"}, Amount: 30, Description: "Third payment description",
				},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
			},
			expected: &domain.Settlement{
				TotalAmount: 2153.52,
				Average:     717.84,
				Debts: []domain.Debt{
					{
						FromID: 1, ToID: 2, Amount: 594.32,
					},
					{
						FromID: 3, ToID: 2, Amount: 687.84,
					},
				},
			},
		},
		{
			name: "SomeTest",
			purchases: []domain.Purchase{
				{
					ID: 1, FundID: 1, Payer: domain.User{ID: 1, Username: "FirstPayer"}, Amount: 123.52, Description: "First payment description",
				},
				{
					ID: 2, FundID: 1, Payer: domain.User{ID: 2, Username: "SecondUser"}, Amount: 2000, Description: "Second payment description",
				},
				{
					ID: 3, FundID: 1, Payer: domain.User{ID: 3, Username: "ThirdUser"}, Amount: 30, Description: "Third payment description",
				},
				{
					ID: 4, FundID: 1, Payer: domain.User{ID: 4, Username: "ThirdUser"}, Amount: 3000, Description: "Third payment description",
				},
			},
			members: []domain.User{
				{ID: 1, Username: "FirstUser"},
				{ID: 2, Username: "SecondUser"},
				{ID: 3, Username: "ThirdUser"},
				{ID: 4, Username: "4th user"},
			},
			expected: &domain.Settlement{TotalAmount: 5153.52, Average: 1288.38, Debts: []domain.Debt{
				{FromID: 1, ToID: 2, Amount: 711.62},
				{FromID: 1, ToID: 4, Amount: 453.24},
				{FromID: 3, ToID: 4, Amount: 1258.38},
			}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settlements := calculateSettlements(tc.purchases, tc.members)
			assert.Equal(t, tc.expected, settlements)
		})
	}
}

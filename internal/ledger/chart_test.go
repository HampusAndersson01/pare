// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package ledger

import "testing"

func TestCoreChartIncludesLegacyMigrationAccounts(t *testing.T) {
	want := map[string]string{
		"1220": "Inventarier och verktyg",
		"2013": "Övriga egna uttag",
		"2018": "Övriga egna insättningar",
		"3301": "Försäljning tjänster inom Sverige, 25 % moms",
		"5090": "Övriga lokalkostnader",
	}

	got := make(map[string]string, len(CoreChart))
	for _, account := range CoreChart {
		got[account.Number] = account.Name
	}

	for number, name := range want {
		if got[number] != name {
			t.Errorf("CoreChart[%s] = %q, want %q", number, got[number], name)
		}
	}
}

package provider

import (
	"fmt"
	"sort"

	"github.com/etax-converter/etax/internal/domain"
	"github.com/etax-converter/etax/internal/provider/ibkr"
)

func Registry() map[string]domain.Provider {
	providers := []domain.Provider{
		ibkr.New(),
	}

	out := make(map[string]domain.Provider, len(providers))
	for _, p := range providers {
		out[p.ID()] = p
	}
	return out
}

func Get(id string) (domain.Provider, error) {
	p, ok := Registry()[id]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q (available: %v)", id, IDs())
	}
	return p, nil
}

func IDs() []string {
	reg := Registry()
	ids := make([]string, 0, len(reg))
	for id := range reg {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

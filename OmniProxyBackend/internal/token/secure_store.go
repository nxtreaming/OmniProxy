package token

import "OmniProxyBackend/internal/securestore"

type secureStore struct {
	inner Store
}

func NewSecureStore(inner Store) Store {
	return secureStore{inner: inner}
}

func (s secureStore) Load() ([]Token, error) {
	items, err := s.inner.Load()
	if err != nil {
		return nil, err
	}

	needsMigration := false
	for i := range items {
		value := items[i].TokenValue
		if value == "" {
			continue
		}
		if !securestore.IsProtectedString(value) {
			needsMigration = true
		}
		plain, err := securestore.UnprotectString(value)
		if err != nil {
			return nil, err
		}
		items[i].TokenValue = plain
	}
	if needsMigration {
		if err := s.Save(items); err != nil {
			return nil, err
		}
	}
	return items, nil
}

func (s secureStore) Save(items []Token) error {
	out := make([]Token, len(items))
	copy(out, items)
	for i := range out {
		value := out[i].TokenValue
		if value == "" {
			continue
		}
		protected, err := securestore.ProtectString(value)
		if err != nil {
			return err
		}
		out[i].TokenValue = protected
	}
	return s.inner.Save(out)
}

package auth

import (
	"context"
	"errors"

	"github.com/khamdanngazis/lovaria/src/platform/seed"
)

// DevPassword adalah password akun seed development.
const DevPassword = "password123"

// Seeder membuat akun development (idempoten):
// couple@lovoria.test dan admin@lovoria.test, password DevPassword.
func Seeder(svc *Service) seed.Seeder {
	return seed.Seeder{
		Name: "auth: akun dev",
		Run: func(ctx context.Context) error {
			accounts := []struct {
				in    RegisterInput
				admin bool
			}{
				{RegisterInput{Name: "Pasangan Demo", Email: "couple@lovoria.test", Password: DevPassword}, false},
				{RegisterInput{Name: "Admin Demo", Email: "admin@lovoria.test", Password: DevPassword}, true},
			}
			for _, a := range accounts {
				var err error
				if a.admin {
					_, err = svc.CreateAdmin(ctx, a.in)
				} else {
					_, err = svc.Register(ctx, a.in)
				}
				var v ValidationError
				if errors.As(err, &v) && v["email"] != "" {
					continue // sudah ada
				}
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
}

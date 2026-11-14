package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"prestamos/internal/customers/adapters/postgres"
	customersapp "prestamos/internal/customers/app"
	customersdomain "prestamos/internal/customers/domain"
	"prestamos/internal/platform/db"
	"prestamos/internal/platform/ids"
	tenantsapp "prestamos/internal/tenants/app"
	tenantsdomain "prestamos/internal/tenants/domain"
)

const (
	demoSlug     = "demo"
	demoPassword = "Demo12345x" // solo desarrollo: nunca en producción (ver run)
)

// seedDemo crea de forma idempotente la empresa demo, un cobrador y clientes de ejemplo.
func seedDemo(ctx context.Context, out io.Writer, runner db.TxRunner, svc *tenantsapp.Service) error {
	t, err := ensureDemoTenant(ctx, out, svc)
	if err != nil {
		return err
	}
	if err := ensureDemoCollector(ctx, svc, t.ID); err != nil {
		return err
	}
	if err := ensureDemoCustomers(ctx, out, runner, t.ID); err != nil {
		return err
	}
	printDemoCredentials(out)
	return nil
}

func ensureDemoTenant(ctx context.Context, out io.Writer, svc *tenantsapp.Service) (*tenantsdomain.Tenant, error) {
	t, err := svc.TenantBySlug(ctx, demoSlug)
	if err == nil {
		fmt.Fprintln(out, "La empresa demo ya existía.")
		return t, nil
	}
	if !errors.Is(err, tenantsdomain.ErrTenantNotFound) {
		return nil, err
	}
	owner := tenantsdomain.NewUser{Email: "owner@demo.local", FullName: "Owner Demo", Password: demoPassword}
	t, _, err = svc.CreateTenant(ctx, "", tenantsdomain.NewTenant{Slug: demoSlug, Name: "Empresa Demo", Owner: owner})
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(out, "Empresa demo creada.")
	return t, nil
}

func ensureDemoCollector(ctx context.Context, svc *tenantsapp.Service, tenantID string) error {
	cobrador := tenantsdomain.NewUser{Email: "cobrador@demo.local", FullName: "Cobrador Demo",
		Password: demoPassword, Roles: []string{"cobrador"}}
	_, err := svc.CreateUser(ctx, "", tenantID, cobrador)
	if errors.Is(err, tenantsdomain.ErrEmailTaken) {
		return nil
	}
	return err
}

func ensureDemoCustomers(ctx context.Context, out io.Writer, runner db.TxRunner, tenantID string) error {
	svc := customersapp.NewService(postgres.NewRepo(runner), ids.UUIDv7{})
	nuevos := 0
	for i, name := range demoNames {
		_, err := svc.Create(ctx, tenantID, customersdomain.Customer{
			FullName:   name,
			DocumentID: fmt.Sprintf("7000%04d", i+1),
			Phone:      fmt.Sprintf("9%08d", i+1),
			Address:    fmt.Sprintf("Jr. Demo %d", i+1),
		})
		switch {
		case err == nil:
			nuevos++
		case errors.Is(err, customersdomain.ErrDuplicateDocument):
			// ya existía de una ejecución anterior
		default:
			return err
		}
	}
	fmt.Fprintf(out, "Clientes demo: %d nuevos, %d ya existían.\n", nuevos, len(demoNames)-nuevos)
	return nil
}

func printDemoCredentials(out io.Writer) {
	fmt.Fprintln(out, "Credenciales DEMO (solo desarrollo):")
	fmt.Fprintf(out, "  Empresa: %s\n", demoSlug)
	fmt.Fprintf(out, "  owner@demo.local / %s (owner)\n", demoPassword)
	fmt.Fprintf(out, "  cobrador@demo.local / %s (cobrador)\n", demoPassword)
}

var demoNames = []string{
	"Juan Pérez", "María Quispe", "Carlos Huamán", "Rosa Mendoza", "Luis Torres",
	"Ana Ruiz", "Pedro Ccama", "Lucía Flores", "Jorge Vargas", "Carmen Rojas",
	"Miguel Ángel Soto", "Elena Paredes", "Ricardo Gómez", "Patricia Luna", "Daniel Ortiz",
	"Sofía Castillo", "Hernán Vega", "Gloria Medina", "Andrés Calle", "Teresa Ramos",
	"Víctor Huanca", "Nelly Chávez", "Oscar Núñez", "Irene Salas", "Raúl Ibarra",
}

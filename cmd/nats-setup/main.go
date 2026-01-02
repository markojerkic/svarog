package main

import (
	"fmt"
	"os"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func main() {
	// 1. Create Operator (The Root of Trust)
	opKp, _ := nkeys.CreateOperator()
	opPub, _ := opKp.PublicKey()
	opClaims := jwt.NewOperatorClaims(opPub)
	opClaims.Name = "Svarog-Operator"
	opJwt, _ := opClaims.Encode(opKp)

	// 2. Create the SYSTEM Account (Required for JetStream)
	sysKp, _ := nkeys.CreateAccount()
	sysPub, _ := sysKp.PublicKey()
	sysClaims := jwt.NewAccountClaims(sysPub)
	sysClaims.Name = "SYS"
	sysJwt, _ := sysClaims.Encode(opKp)

	// 3. Create the APP Account (Main data account)
	accKp, _ := nkeys.CreateAccount()
	accPub, _ := accKp.PublicKey()
	accSeed, _ := accKp.Seed()
	accClaims := jwt.NewAccountClaims(accPub)
	accClaims.Name = "APP"
	accClaims.Limits.JetStreamLimits.DiskStorage = -1
	accClaims.Limits.JetStreamLimits.MemoryStorage = -1
	accJwt, _ := accClaims.Encode(opKp)

	// 4. Create the "Admin" User (Inside the APP account)
	adminKp, _ := nkeys.CreateUser()
	adminPub, _ := adminKp.PublicKey()
	adminSeed, _ := adminKp.Seed()
	adminClaims := jwt.NewUserClaims(adminPub)
	adminClaims.Name = "admin"
	adminClaims.IssuerAccount = accPub
	adminClaims.Permissions.Pub.Allow.Add(">")
	adminClaims.Permissions.Sub.Allow.Add(">")
	adminJwt, _ := adminClaims.Encode(accKp)

	// Save admin.creds for your backend to connect
	adminCreds, _ := jwt.FormatUserConfig(adminJwt, adminSeed)
	os.WriteFile("admin.creds", adminCreds, 0600)

	// Output for .env
	fmt.Println("# --- COPY TO .env ---")
	fmt.Printf("NATS_OPERATOR_JWT=%s\n", opJwt)
	fmt.Printf("NATS_ACCOUNT_PUBLIC_KEY=%s\n", accPub)
	fmt.Printf("NATS_ACCOUNT_JWT=%s\n", accJwt)
	fmt.Printf("NATS_ACCOUNT_SEED=%s\n", accSeed)

	// Output for nats-server.conf
	fmt.Println("\n# --- COPY TO nats-server.conf ---")
	fmt.Printf(`
operator: %s
system_account: %s

resolver: MEMORY
resolver_preload: {
    %s: %s,
    %s: %s
}
`, opJwt, sysPub, accPub, accJwt, sysPub, sysJwt)

	fmt.Println("\nSuccess! 'admin.creds' file created.")
}

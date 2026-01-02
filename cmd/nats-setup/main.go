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
	opJwt, _ := opClaims.Encode(opKp)

	// 2. Create the Main Account
	accKp, _ := nkeys.CreateAccount()
	accPub, _ := accKp.PublicKey()
	accSeed, _ := accKp.Seed()
	accClaims := jwt.NewAccountClaims(accPub)
	accClaims.Name = "APP"
	// Enable JetStream limits (Unlimited)
	accClaims.Limits.JetStreamLimits.DiskStorage = -1
	accClaims.Limits.JetStreamLimits.MemoryStorage = -1
	accJwt, _ := accClaims.Encode(opKp)

	// 3. Create the "Server Admin" User (Replaces user/pass)
	// This user has FULL permissions to do anything.
	adminKp, _ := nkeys.CreateUser()
	adminPub, _ := adminKp.PublicKey()
	adminSeed, _ := adminKp.Seed()
	adminClaims := jwt.NewUserClaims(adminPub)
	adminClaims.Name = "admin"
	adminClaims.IssuerAccount = accPub
	adminClaims.Permissions.Pub.Allow.Add(">") // Allow All
	adminClaims.Permissions.Sub.Allow.Add(">") // Allow All
	adminJwt, _ := adminClaims.Encode(accKp)

	// Generate the .creds file for the Backend to use
	adminCreds, _ := jwt.FormatUserConfig(adminJwt, adminSeed)
	os.WriteFile("admin.creds", adminCreds, 0600)

	// 4. Output the .env content for Docker
	fmt.Println("# --- COPY TO .env ---")
	fmt.Printf("NATS_OPERATOR_JWT=%s\n", opJwt)
	fmt.Printf("NATS_ACCOUNT_PUBLIC_KEY=%s\n", accPub)
	fmt.Printf("NATS_ACCOUNT_JWT=%s\n", accJwt)
	fmt.Println("# --- KEEP SECRET (IN BACKEND) ---")
	fmt.Printf("NATS_ACCOUNT_SEED=%s\n", accSeed)
	fmt.Println("\nSuccess! 'admin.creds' file created.")

	fmt.Println("# --- COPY TO nats-server.conf ---")
	fmt.Printf(
		`operator: %s
resolver: MEMORY
resolver_preload: {
  %s: %s
  %s: %s
}
`, opJwt, accPub, accJwt, adminPub, adminJwt)

}

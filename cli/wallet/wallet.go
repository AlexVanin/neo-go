package wallet

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	errNoPath         = errors.New("target path where the wallet should be stored is mandatory and should be passed using (--path, -p) flags")
	errPhraseMismatch = errors.New("the entered pass-phrases do not match. Maybe you have misspelled them")
)

var (
	walletPathFlag = cli.StringFlag{
		Name:  "path, p",
		Usage: "Target location of the wallet file.",
	}
	wifFlag = cli.StringFlag{
		Name:  "wif",
		Usage: "WIF to import",
	}
	decryptFlag = cli.BoolFlag{
		Name:  "decrypt, d",
		Usage: "Decrypt encrypted keys.",
	}
	rpcFlag = cli.StringFlag{
		Name:  "rpc, r",
		Usage: "RPC node address",
	}
	timeoutFlag = cli.DurationFlag{
		Name:  "timeout, t",
		Usage: "Timeout for the operation",
	}
)

// NewCommands returns 'wallet' command.
func NewCommands() []cli.Command {
	return []cli.Command{{
		Name:  "wallet",
		Usage: "create, open and manage a NEO wallet",
		Subcommands: []cli.Command{
			{
				Name:   "claim",
				Usage:  "claim GAS",
				Action: claimGas,
				Flags: []cli.Flag{
					walletPathFlag,
					rpcFlag,
					timeoutFlag,
					cli.StringFlag{
						Name:  "address, a",
						Usage: "Address to claim GAS for",
					},
				},
			},
			{
				Name:   "create",
				Usage:  "create a new wallet",
				Action: createWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					cli.BoolFlag{
						Name:  "account, a",
						Usage: "Create a new account",
					},
				},
			},
			{
				Name:   "create-account",
				Usage:  "add an account to the existing wallet",
				Action: addAccount,
				Flags: []cli.Flag{
					walletPathFlag,
				},
			},
			{
				Name:   "dump",
				Usage:  "check and dump an existing NEO wallet",
				Action: dumpWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					decryptFlag,
				},
			},
			{
				Name:      "export",
				Usage:     "export keys for address",
				UsageText: "export --path <path> [--decrypt] [<address>]",
				Action:    exportKeys,
				Flags: []cli.Flag{
					walletPathFlag,
					decryptFlag,
				},
			},
			{
				Name:   "import",
				Usage:  "import WIF",
				Action: importWallet,
				Flags: []cli.Flag{
					walletPathFlag,
					wifFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
				},
			},
			{
				Name:  "import-multisig",
				Usage: "import multisig contract",
				UsageText: "import-multisig --path <path> --wif <wif> --min <n>" +
					" [<pubkey1> [<pubkey2> [...]]]",
				Action: importMultisig,
				Flags: []cli.Flag{
					walletPathFlag,
					wifFlag,
					cli.StringFlag{
						Name:  "name, n",
						Usage: "Optional account name",
					},
					cli.IntFlag{
						Name:  "min, m",
						Usage: "Minimal number of signatures",
					},
				},
			},
			{
				Name:  "transfer",
				Usage: "transfer NEO/GAS",
				UsageText: "transfer --path <path> --from <addr> --to <addr>" +
					" --amount <amount> --asset [NEO|GAS|<hex-id>]",
				Action: transferAsset,
				Flags: []cli.Flag{
					walletPathFlag,
					rpcFlag,
					timeoutFlag,
					cli.StringFlag{
						Name:  "from",
						Usage: "Address to send an asset from",
					},
					cli.StringFlag{
						Name:  "to",
						Usage: "Address to send an asset to",
					},
					cli.StringFlag{
						Name:  "amount",
						Usage: "Amount of asset to send",
					},
					cli.StringFlag{
						Name:  "asset",
						Usage: "Asset ID",
					},
				},
			},
		},
	}}
}

func claimGas(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	addr := ctx.String("address")
	scriptHash, err := address.StringToUint160(addr)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	acc := wall.GetAccount(scriptHash)
	if acc == nil {
		return cli.NewExitError(fmt.Errorf("wallet contains no account for '%s'", addr), 1)
	}

	pass, err := readPassword("Enter password > ")
	if err != nil {
		return cli.NewExitError(err, 1)
	} else if err := acc.Decrypt(pass); err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := getGoContext(ctx)
	defer cancel()

	c, err := client.New(gctx, ctx.String("rpc"), client.Options{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	info, err := c.GetClaimable(addr)
	if err != nil {
		return cli.NewExitError(err, 1)
	} else if info.Unclaimed == 0 || len(info.Spents) == 0 {
		fmt.Println("Nothing to claim")
		return nil
	}

	var claim transaction.ClaimTX
	for i := range info.Spents {
		claim.Claims = append(claim.Claims, transaction.Input{
			PrevHash:  info.Spents[i].Tx,
			PrevIndex: uint16(info.Spents[i].N),
		})
	}

	tx := &transaction.Transaction{
		Type: transaction.ClaimType,
		Data: &claim,
	}

	tx.AddOutput(&transaction.Output{
		AssetID:    core.UtilityTokenID(),
		Amount:     info.Unclaimed,
		ScriptHash: scriptHash,
	})

	_ = acc.SignTx(tx)
	if err := c.SendRawTransaction(tx); err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Println(tx.Hash().StringLE())
	return nil
}

func addAccount(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	if err := createAccount(ctx, wall); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func exportKeys(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	var addr string

	decrypt := ctx.Bool("decrypt")
	if ctx.NArg() == 0 && decrypt {
		return cli.NewExitError(errors.New("address must be provided if '--decrypt' flag is used"), 1)
	} else if ctx.NArg() > 0 {
		// check address format just to catch possible typos
		addr = ctx.Args().First()
		_, err := address.StringToUint160(addr)
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't parse address: %v", err), 1)
		}
	}

	var wifs []string

loop:
	for _, a := range wall.Accounts {
		if addr != "" && a.Address != addr {
			continue
		}

		for i := range wifs {
			if a.EncryptedWIF == wifs[i] {
				continue loop
			}
		}

		wifs = append(wifs, a.EncryptedWIF)
	}

	for _, wif := range wifs {
		if decrypt {
			pass, err := readPassword("Enter password > ")
			if err != nil {
				return cli.NewExitError(err, 1)
			}

			pk, err := keys.NEP2Decrypt(wif, pass)
			if err != nil {
				return cli.NewExitError(err, 1)
			}

			wif = pk.WIF()
		}

		fmt.Println(wif)
	}

	return nil
}

func importMultisig(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	m := ctx.Int("min")
	if ctx.NArg() < m {
		return cli.NewExitError(errors.New("insufficient number of public keys"), 1)
	}

	args := []string(ctx.Args())
	pubs := make([]*keys.PublicKey, len(args))

	for i := range args {
		pubs[i], err = keys.NewPublicKeyFromString(args[i])
		if err != nil {
			return cli.NewExitError(fmt.Errorf("can't decode public key %d: %v", i, err), 1)
		}
	}

	acc, err := newAccountFromWIF(ctx.String("wif"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if err := acc.ConvertMultisig(m, pubs); err != nil {
		return cli.NewExitError(err, 1)
	}

	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func importWallet(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	defer wall.Close()

	acc, err := newAccountFromWIF(ctx.String("wif"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	acc.Label = ctx.String("name")
	if err := addAccountAndSave(wall, acc); err != nil {
		return cli.NewExitError(err, 1)
	}

	return nil
}

func transferAsset(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	from := ctx.String("from")
	addr, err := address.StringToUint160(from)
	if err != nil {
		return cli.NewExitError("invalid address", 1)
	}
	acc := wall.GetAccount(addr)
	if acc == nil {
		return cli.NewExitError(fmt.Errorf("wallet contains no account for '%s'", addr), 1)
	}

	asset, err := getAssetID(ctx.String("asset"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid asset id: %v", err), 1)
	}

	amount, err := util.Fixed8FromString(ctx.String("amount"))
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid amount: %v", err), 1)
	}

	pass, err := readPassword("Enter wallet password > ")
	if err != nil {
		return cli.NewExitError(err, 1)
	} else if err := acc.Decrypt(pass); err != nil {
		return cli.NewExitError(err, 1)
	}

	gctx, cancel := getGoContext(ctx)
	defer cancel()

	c, err := client.New(gctx, ctx.String("rpc"), client.Options{})
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx := transaction.NewContractTX()
	if err := request.AddInputsAndUnspentsToTx(tx, from, asset, amount, c); err != nil {
		return cli.NewExitError(err, 1)
	}

	toAddr, err := address.StringToUint160(ctx.String("to"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	tx.AddOutput(&transaction.Output{
		AssetID:    asset,
		Amount:     amount,
		ScriptHash: toAddr,
		Position:   1,
	})

	_ = acc.SignTx(tx)
	if err := c.SendRawTransaction(tx); err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Println(tx.Hash().StringLE())
	return nil
}

func getGoContext(ctx *cli.Context) (context.Context, func()) {
	if dur := ctx.Duration("timeout"); dur != 0 {
		return context.WithTimeout(context.Background(), dur)
	}
	return context.Background(), func() {}
}

func dumpWallet(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("path"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if ctx.Bool("decrypt") {
		pass, err := readPassword("Enter wallet password > ")
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		for i := range wall.Accounts {
			// Just testing the decryption here.
			err := wall.Accounts[i].Decrypt(pass)
			if err != nil {
				return cli.NewExitError(err, 1)
			}
		}
	}
	fmtPrintWallet(wall)
	return nil
}

func createWallet(ctx *cli.Context) error {
	path := ctx.String("path")
	if len(path) == 0 {
		return cli.NewExitError(errNoPath, 1)
	}
	wall, err := wallet.NewWallet(path)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if err := wall.Save(); err != nil {
		return cli.NewExitError(err, 1)
	}

	if ctx.Bool("account") {
		if err := createAccount(ctx, wall); err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	fmtPrintWallet(wall)
	fmt.Printf("wallet successfully created, file location is %s\n", wall.Path())
	return nil
}

func readAccountInfo() (string, string, error) {
	buf := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the name of the account > ")
	rawName, _ := buf.ReadBytes('\n')
	phrase, err := readPassword("Enter passphrase > ")
	if err != nil {
		return "", "", err
	}
	phraseCheck, err := readPassword("Confirm passphrase > ")
	if err != nil {
		return "", "", err
	}

	if phrase != phraseCheck {
		return "", "", errPhraseMismatch
	}

	name := strings.TrimRight(string(rawName), "\n")
	return name, phrase, nil
}

func createAccount(ctx *cli.Context, wall *wallet.Wallet) error {
	name, phrase, err := readAccountInfo()
	if err != nil {
		return err
	}
	return wall.CreateAccount(name, phrase)
}

func openWallet(path string) (*wallet.Wallet, error) {
	if len(path) == 0 {
		return nil, errNoPath
	}
	return wallet.NewWalletFromFile(path)
}

func getAssetID(s string) (util.Uint256, error) {
	s = strings.ToLower(s)
	switch {
	case s == "neo":
		return core.GoverningTokenID(), nil
	case s == "gas":
		return core.UtilityTokenID(), nil
	default:
		return util.Uint256DecodeStringLE(s)
	}
}

func newAccountFromWIF(wif string) (*wallet.Account, error) {
	// note: NEP2 strings always have length of 58 even though
	// base58 strings can have different lengths even if slice lengths are equal
	if len(wif) == 58 {
		pass, err := readPassword("Enter password > ")
		if err != nil {
			return nil, err
		}

		return wallet.NewAccountFromEncryptedWIF(wif, pass)
	}

	acc, err := wallet.NewAccountFromWIF(wif)
	if err != nil {
		return nil, err
	}

	fmt.Println("Provided WIF was unencrypted. Wallet can contain only encrypted keys.")
	name, pass, err := readAccountInfo()
	if err != nil {
		return nil, err
	}

	acc.Label = name
	if err := acc.Encrypt(pass); err != nil {
		return nil, err
	}

	return acc, nil
}

func addAccountAndSave(w *wallet.Wallet, acc *wallet.Account) error {
	for i := range w.Accounts {
		if w.Accounts[i].Address == acc.Address {
			return fmt.Errorf("address '%s' is already in wallet", acc.Address)
		}
	}

	w.AddAccount(acc)
	return w.Save()
}

func readPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	rawPass, err := terminal.ReadPassword(syscall.Stdin)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(rawPass), "\n"), nil
}

func fmtPrintWallet(wall *wallet.Wallet) {
	b, _ := wall.JSON()
	fmt.Println("")
	fmt.Println(string(b))
	fmt.Println("")
}

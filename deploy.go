package solar

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/pkg/errors"
)

func init() {
	cmd := app.Command("deploy", "Compile Solidity contracts.")

	targets := cmd.Arg("targets", "Solidity contracts to deploy.").Strings()

	appTasks["deploy"] = func() (err error) {
		// verify before deploy
		log.Println("env", *solarEnv)
		log.Println("rpc", *solarRPC)

		targets := *targets

		if len(targets) == 0 {
			return errors.New("nothing to deploy")
		}

		deployer, err := NewDeployer("solar.json")
		if err != nil {
			return
		}
		deployer.RPCHost = *solarRPC

		for _, target := range targets {
			dt := parseDeployTarget(target)

			pretty.Println("deploy", dt)
			err := deployer.CreateContract(dt.Name, dt.FilePath)
			if err != nil {
				fmt.Println("deploy:", err)
			}

			// TODO: verify no duplicate target names
			// TODO: verify all contracts before deploy
		}

		return
	}
}

type deployTarget struct {
	Name     string
	FilePath string
}

func parseDeployTarget(target string) deployTarget {
	parts := strings.Split(target, ":")

	filepath := parts[0]

	var name string
	if len(parts) == 2 {
		name = parts[1]
	} else {
		name = stringLowerFirstRune(basenameNoExt(filepath))
	}

	// TODO verify name for valid JS name

	t := deployTarget{
		Name:     name,
		FilePath: filepath,
	}

	return t
}

type Deployer struct {
	Env     string
	RPCHost string

	repo *deployedContractsRepository
}

func NewDeployer(repoFile string) (*Deployer, error) {
	repo, err := openDeployedContractsRepository("solar.json")
	if err != nil {
		return nil, err
	}

	return &Deployer{
		repo: repo,
	}, nil
}

func (d *Deployer) CreateContract(name, filepath string) (err error) {
	gasLimit := 300000

	rpcURL, err := url.Parse(d.RPCHost)
	if err != nil {
		return errors.Wrap(err, "rpc host")
	}

	rpc := qtumRPC{rpcURL}

	contract, err := compileSource(filepath, CompilerOptions{})
	if err != nil {
		return errors.Wrap(err, "compile")
	}

	var tx TransactionReceipt

	err = rpc.Call(&tx, "createcontract", contract.Bin.String(), gasLimit)

	if err != nil {
		return errors.Wrap(err, "createcontract")
	}

	fmt.Println("tx", tx.Address)
	fmt.Println("contract name", contract.Name)

	deployedContract := DeployedContract{
		Name:             contract.Name,
		CompiledContract: *contract,
		TransactionID:    tx.TxID,
		Address:          tx.Address,
		CreatedAt:        time.Now(),
	}

	err = d.repo.Add(name, deployedContract)
	if err != nil {
		return
	}

	err = d.repo.Commit()
	if err != nil {
		return
	}
	// pretty.Println("rpc err", string(res.RawError))

	return nil
}

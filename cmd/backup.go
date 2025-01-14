package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/arenadata/adcm-installer/compose"
	"github.com/arenadata/adcm-installer/models"

	"github.com/containerd/platforms"
	"github.com/docker/cli/cli/command"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"path/filepath"
)

const backupDir = "/backup"

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup ADCM files and DB",
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.WithField("command", "backup")

		deployId, _ := cmd.Flags().GetString("deployment-id")
		comp, err := compose.NewComposeService(
			command.WithOutputStream(cmd.OutOrStdout()),
			command.WithErrorStream(cmd.ErrOrStderr()),
		)
		if err != nil {
			logger.Fatal(err)
		}

		outputDir, _ := cmd.Flags().GetString("output-dir")
		outputDir, err = filepath.Abs(outputDir)
		if err != nil {
			logger.Fatal(err)
		}

		if err = startBackup(cmd.Context(), comp, deployId, outputDir, logger); err != nil {
			logger.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringP("deployment-id", "d", "", "Set specific deployment name (ID)")
	_ = backupCmd.MarkFlagRequired("deployment-id")
	backupCmd.Flags().StringP("output-dir", "o", "", "Set output directory")
}

func startBackup(ctx context.Context, comp *compose.Compose, deployId, outputDir string, logger *log.Entry) error {
	//TODO: need refactoring
	prj, err := comp.GetProject(ctx, deployId)
	if err != nil {
		return err
	}

	svc, err := prj.GetService(models.PostgresServiceName)
	if err != nil {
		return err
	}

	envs := svc.Environment.ToMapping().Values()
	envs = append(envs, fmt.Sprintf("PGHOST=%s", models.PostgresServiceName))
	if v, ok := svc.Environment["POSTGRES_USER"]; !ok {
		return fmt.Errorf("POSTGRES_USER not found in environment")
	} else {
		envs = append(envs, fmt.Sprintf("PGUSER=%s", *v))
	}
	if v, ok := svc.Environment["POSTGRES_PASSWORD"]; !ok {
		return fmt.Errorf("POSTGRES_PASSWORD not found in environment")
	} else {
		envs = append(envs, fmt.Sprintf("PGPASSWORD=%s", *v))
	}

	if err = comp.Pause(ctx, deployId, models.ADCMServiceName); err != nil {
		return err
	}
	defer func() {
		if e := comp.UnPause(ctx, deployId, models.ADCMServiceName); e != nil {
			err = e
		}
	}()

	containerConfig := &containerTypes.Config{
		Image:      svc.Image,
		WorkingDir: backupDir,
		Env:        envs,
		Entrypoint: []string{"/bin/sh"},
		Tty:        true,
	}

	hostConfig := &containerTypes.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: outputDir,
				Target: backupDir,
			},
		},
		VolumesFrom: []string{compose.ContainerName(deployId, models.ADCMServiceName)},
	}

	networkNames := svc.NetworksByPriority()
	networkConfig := &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{
		networkNames[0]: {},
	}}

	platform, err := platforms.Parse(models.DefaultPlatform)
	if err != nil {
		return err
	}

	backupContainerName := compose.ContainerName(deployId, "backup")
	logger.Infof("Create container %s ...", backupContainerName)
	err = comp.ContainerRun(ctx, platform, hostConfig, networkConfig, containerConfig, backupContainerName)
	if err != nil {
		return err
	}
	defer func() {
		if e := comp.ContainerRemove(ctx, backupContainerName); e != nil {
			logger.Warnf("Failed to remove backup container %s: %v", backupContainerName, e)
		}
	}()

	const adcmBackupDir = "adcm-backup"
	// TODO: use context path
	if err = comp.Exec(ctx, backupContainerName, "mkdir", "-p", adcmBackupDir); err != nil {
		return err
	}
	defer func() {
		if e := comp.Exec(ctx, backupContainerName, "rm", "-rf", adcmBackupDir); e != nil {
			logger.Warnf("Failed to remove temporay backup directory: %v", e)
		}
	}()

	logger.Infof("Backup database ...")
	if err = comp.Exec(ctx, backupContainerName, "pg_dumpall", "-f", filepath.Join(adcmBackupDir, "dump.sql")); err != nil {
		return fmt.Errorf(fmt.Sprintf("process exit with error: %v", err))
	}

	logger.Infof("Backup ADCM files ...")
	if err = comp.Exec(ctx, backupContainerName, "cp", "-a", "/adcm/data", filepath.Join(adcmBackupDir, "data")); err != nil {
		return fmt.Errorf("process2 exit with error: %v", err)
	}

	fileName := fmt.Sprintf("adcm-backup-%s.tgz", time.Now().Format("20060102150405"))

	logger.Infof("Create tar archive ...")
	if err = comp.Exec(ctx, backupContainerName, "tar", "-czf", fileName, adcmBackupDir); err != nil {
		return fmt.Errorf("process2 exit with error: %v", err)
	}

	logger.Infof("Backup finished")

	return err
}

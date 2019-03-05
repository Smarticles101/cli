package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/exercism/cli/api"
	"github.com/exercism/cli/browser"
	"github.com/exercism/cli/config"
	"github.com/exercism/cli/workspace"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// openCmd opens the designated exercise in the browser.
var openCmd = &cobra.Command{
	Use:     "open",
	Aliases: []string{"o"},
	Short:   "Open an exercise on the website.",
	Long: `Open the specified exercise to the solution page on the Exercism website.

Pass the path to the directory that contains the solution you want to see on the website.
	`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.NewConfig()

		v := viper.New()
		v.AddConfigPath(cfg.Dir)
		v.SetConfigName("user")
		v.SetConfigType("json")
		// Ignore error. If the file doesn't exist, that is fine.
		_ = v.ReadInConfig()
		cfg.UserViperConfig = v

		return runOpen(cfg, cmd.Flags(), args)
	},
}

func runOpen(cfg config.Config, flags *pflag.FlagSet, args []string) error {
	var url string
	usrCfg := cfg.UserViperConfig
	trackID, _ := flags.GetString("track")
	exerciseID, _ := flags.GetString("exercise")

	if exerciseID == "" {
		return fmt.Errorf("Must provide an `--exercise`")
	}

	if remote, _ := flags.GetBool("remote"); remote {
		apiUrl := fmt.Sprintf("%s/solutions/latest", usrCfg.GetString("apibaseurl"))

		client, err := api.NewClient(usrCfg.GetString("token"), usrCfg.GetString("apibaseurl"))
		if err != nil {
			return err
		}

		req, err := client.NewRequest("GET", apiUrl, nil)
		if err != nil {
			return err
		}

		q := req.URL.Query()
		q.Add("exercise_id", exerciseID)
		if trackID != "" {
			q.Add("track_id", trackID)
		}
		req.URL.RawQuery = q.Encode()

		res, err := client.Do(req)
		if err != nil {
			return err
		}

		defer res.Body.Close()
		var payload openPayload
		if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
			return fmt.Errorf("unable to parse API response - %s", err)
		}

		if res.StatusCode != http.StatusOK {
			switch payload.Error.Type {
			case "track_ambiguous":
				return fmt.Errorf("%s: %s", payload.Error.Message, strings.Join(payload.Error.PossibleTrackIDs, ", "))
			default:
				return errors.New(payload.Error.Message)
			}
		}

		url = payload.Solution.URL
	} else {
		ws, err := workspace.New(usrCfg.GetString("workspace"))
		if err != nil {
			return err
		}
		exercises, err := ws.Exercises()
		if err != nil {
			return err
		}

		matchingExercises := make([]workspace.Exercise, 0, len(exercises))
		for _, exercise := range exercises {
			if trackID != "" {
				if exercise.Track == trackID && exercise.Slug == exerciseID {
					matchingExercises = append(matchingExercises, exercise)
				}
			} else if exercise.Slug == exerciseID {
				matchingExercises = append(matchingExercises, exercise)
			}
		}

		switch len(matchingExercises) {
		case 0:
			return fmt.Errorf("No matching exercise found")
		case 1:
			metaDir := matchingExercises[0].MetadataDir()
			meta, err := workspace.NewExerciseMetadata(metaDir)

			if err != nil {
				return err
			}

			url = meta.URL
			break
		default:
			tracks := make([]string, 0, len(matchingExercises))
			for _, exercise := range matchingExercises {
				tracks = append(tracks, exercise.Track)
			}
			return fmt.Errorf("Please specify a track ID: %s", strings.Join(tracks, ", "))
		}
	}
	browser.Open(url)
	return nil
}

type openPayload struct {
	Solution struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"solution"`
	Error struct {
		Type             string   `json:"type"`
		Message          string   `json:"message"`
		PossibleTrackIDs []string `json:"possible_track_ids"`
	} `json:"error,omitempty"`
}

func setupOpenFlags(flags *pflag.FlagSet) {
	flags.BoolP("remote", "r", false, "checks for remote solutions")
	flags.StringP("track", "t", "", "the track id")
	flags.StringP("exercise", "e", "", "the exercise slug")
}

func init() {
	RootCmd.AddCommand(openCmd)
	setupOpenFlags(openCmd.Flags())
}

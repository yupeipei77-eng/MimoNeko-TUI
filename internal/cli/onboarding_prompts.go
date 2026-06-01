package cli

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	surveyterm "github.com/AlecAivazis/survey/v2/terminal"
	"github.com/mimoneko/mimoneko/internal/auth"
	"golang.org/x/term"
)

const customModelOption = "Custom..."

func promptOnboardingProvider(reader *bufio.Reader, env Env) (string, error) {
	if opts, ok := surveyAskOptions(env); ok {
		answer := "MiMo"
		err := survey.AskOne(&survey.Select{
			Message: "Provider",
			Options: []string{"MiMo", "OpenAI-compatible", "Local"},
			Default: "MiMo",
		}, &answer, opts...)
		if err != nil {
			return "", err
		}
		return normalizeOnboardingProvider(answer), nil
	}

	fmt.Fprintln(env.Stdout, "1. MiMo (recommended)")
	fmt.Fprintln(env.Stdout, "2. OpenAI-compatible")
	fmt.Fprintln(env.Stdout, "3. Local")
	provider := normalizeOnboardingProvider(promptOnboardingLine(reader, env, "Select", "1"))
	if provider == "" {
		return "", fmt.Errorf("unsupported provider selection")
	}
	return provider, nil
}

func promptOnboardingSecret(reader *bufio.Reader, env Env, prompt string) (string, error) {
	if opts, ok := surveyAskOptions(env); ok {
		answer := ""
		err := survey.AskOne(&survey.Password{
			Message: prompt,
		}, &answer, opts...)
		return answer, err
	}
	return promptSecretLine(reader, env, prompt), nil
}

func promptOnboardingInput(reader *bufio.Reader, env Env, prompt string, defaultValue string) (string, error) {
	if opts, ok := surveyAskOptions(env); ok {
		answer := defaultValue
		err := survey.AskOne(&survey.Input{
			Message: prompt,
			Default: defaultValue,
		}, &answer, opts...)
		return answer, err
	}
	return promptOnboardingLine(reader, env, prompt, defaultValue), nil
}

func promptOnboardingModel(reader *bufio.Reader, env Env, provider string) (string, error) {
	defaultModel := auth.DefaultModel(provider)
	if opts, ok := surveyAskOptions(env); ok {
		answer := defaultModel
		err := survey.AskOne(&survey.Select{
			Message: "Model",
			Options: onboardingModelOptions(provider),
			Default: defaultModel,
		}, &answer, opts...)
		if err != nil {
			return "", err
		}
		if answer == customModelOption {
			return promptOnboardingInput(reader, env, "Custom Model", defaultModel)
		}
		return answer, nil
	}
	return promptOnboardingLine(reader, env, "Model", defaultModel), nil
}

func onboardingModelOptions(provider string) []string {
	defaultModel := auth.DefaultModel(provider)
	return []string{defaultModel, customModelOption}
}

func surveyAskOptions(env Env) ([]survey.AskOpt, bool) {
	in, inOK := env.Stdin.(surveyterm.FileReader)
	out, outOK := env.Stdout.(surveyterm.FileWriter)
	if !inOK || !outOK {
		return nil, false
	}
	if !term.IsTerminal(int(in.Fd())) || !term.IsTerminal(int(out.Fd())) {
		return nil, false
	}

	errWriter := env.Stderr
	if errWriter == nil {
		errWriter = env.Stdout
	}
	focusIcon := ">"
	if SupportsEmoji() {
		focusIcon = "\u276f"
	}
	return []survey.AskOpt{
		survey.WithStdio(in, out, errWriter),
		survey.WithPageSize(5),
		survey.WithIcons(func(icons *survey.IconSet) {
			icons.SelectFocus.Text = focusIcon
		}),
	}, true
}

func handleOnboardingPromptError(env Env, err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, surveyterm.InterruptErr) {
		PrintWarning(env.Stdout, "Setup cancelled.")
		return 1
	}
	PrintErrorDetails(env.Stderr, "Setup failed", "Unable to read setup input.", "Run: mimoneko auth login", err.Error())
	return 1
}

func displayOnboardingProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "mimo":
		return "MiMo"
	case "openai":
		return "OpenAI-compatible"
	case "local":
		return "Local"
	default:
		return provider
	}
}

func waitForOnboardingEnter(reader *bufio.Reader) {
	if reader == nil {
		return
	}
	_, _ = reader.ReadString('\n')
}

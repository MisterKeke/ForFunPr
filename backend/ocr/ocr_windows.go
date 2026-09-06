//go:build windows

package ocr

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type windowsEngine struct{}

func New() Engine                     { return windowsEngine{} }
func (windowsEngine) Supported() bool { return true }

const windowsOCRScript = `$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Add-Type -AssemblyName System.Runtime.WindowsRuntime
[Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType=WindowsRuntime] | Out-Null
[Windows.Storage.StorageFile, Windows.Storage, ContentType=WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation, ContentType=WindowsRuntime] | Out-Null
$genericAsTask = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
  $_.Name -eq 'AsTask' -and $_.IsGenericMethod -and $_.GetParameters().Count -eq 1
})[0]
function Await-Result($operation, [Type]$resultType) {
  $task = $genericAsTask.MakeGenericMethod($resultType).Invoke($null, @($operation))
  $task.Wait()
  return $task.Result
}
$file = Await-Result ([Windows.Storage.StorageFile]::GetFileFromPathAsync($env:SOMETHING_OCR_IMAGE)) ([Windows.Storage.StorageFile])
$stream = Await-Result ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStreamWithContentType])
try {
  $decoder = Await-Result ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
  $bitmap = Await-Result ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
  try {
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
    if ($null -eq $engine) { throw 'No Windows OCR language is installed.' }
    $result = Await-Result ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
    Write-Output ('LANGUAGE:' + $engine.RecognizerLanguage.LanguageTag)
    Write-Output 'TEXT:'
    Write-Output $result.Text
  } finally {
    if ($null -ne $bitmap) { $bitmap.Dispose() }
  }
} finally {
  $stream.Dispose()
}`

func (windowsEngine) Recognize(ctx context.Context, imagePath string) (string, string, error) {
	command := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Sta", "-ExecutionPolicy", "Bypass", "-Command", windowsOCRScript)
	command.Env = append(os.Environ(), "SOMETHING_OCR_IMAGE="+imagePath)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 800 {
			message = message[:800]
		}
		if message == "" {
			message = err.Error()
		}
		return "", "", fmt.Errorf("Windows OCR failed: %s", message)
	}
	value := strings.ReplaceAll(string(output), "\r\n", "\n")
	lines := strings.Split(value, "\n")
	language := ""
	textStart := -1
	for index, line := range lines {
		if strings.HasPrefix(line, "LANGUAGE:") {
			language = strings.TrimSpace(strings.TrimPrefix(line, "LANGUAGE:"))
		}
		if line == "TEXT:" {
			textStart = index + 1
			break
		}
	}
	if textStart < 0 {
		return "", "", fmt.Errorf("Windows OCR returned an unexpected response")
	}
	return strings.TrimSpace(strings.Join(lines[textStart:], "\n")), language, nil
}

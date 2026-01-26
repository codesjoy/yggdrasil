#!/usr/bin/env pwsh
<#
 Copyright 2022 The codesjoy Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
#>


<#
.SYNOPSIS
    xDS Basic Example Test Script

.DESCRIPTION
    Automated test script for the xDS basic integration example.
    This script starts the xDS control plane, server, and client,
    then verifies the expected output.

.PARAMETER ControlPlanePath
    Path to the control-plane directory (default: ../control-plane)

.PARAMETER ServerPath
    Path to the server directory (default: ./server)

.PARAMETER ClientPath
    Path to the client directory (default: ./client)

.PARAMETER Timeout
    Maximum time to wait for the test to complete (default: 30 seconds)
#>

param(
    [string]$ControlPlanePath = "../control-plane",
    [string]$ServerPath = "./server",
    [string]$ClientPath = "./client",
    [int]$Timeout = 30
)

$ErrorActionPreference = "Stop"

function Write-ColorOutput {
    param(
        [string]$Message,
        [string]$Color = "Green"
    )
    Write-Host $Message -ForegroundColor $Color
}

function Test-Command {
    param(
        [string]$Command
    )
    try {
        $null = Get-Command $Command -ErrorAction Stop
        return $true
    }
    catch {
        return $false
    }
}

function Start-ProcessWithTimeout {
    param(
        [string]$Path,
        [string]$Arguments,
        [int]$TimeoutSeconds,
        [string]$Name
    )
    
    $process = Start-Process -FilePath $Path -ArgumentList $Arguments -NoNewWindow -PassThru -RedirectStandardOutput "$Name.stdout.log" -RedirectStandardError "$Name.stderr.log"
    
    $startTime = Get-Date
    $timedOut = $false
    
    while (-not $process.HasExited) {
        $elapsed = (Get-Date) - $startTime
        if ($elapsed.TotalSeconds -gt $TimeoutSeconds) {
            Write-ColorOutput "Timeout waiting for $Name ($TimeoutSeconds seconds)" "Red"
            $process.Kill()
            $timedOut = $true
            break
        }
        Start-Sleep -Milliseconds 100
    }
    
    if ($timedOut) {
        return $false
    }
    
    if ($process.ExitCode -ne 0) {
        Write-ColorOutput "$Name exited with code $($process.ExitCode)" "Red"
        Write-ColorOutput "Error output:" "Red"
        Get-Content "$Name.stderr.log" | Write-Host
        return $false
    }
    
    return $true
}

function Cleanup {
    Write-ColorOutput "`nCleaning up..." "Yellow"
    
    Get-Process | Where-Object {
        $_.ProcessName -like "*control-plane*" -or
        $_.ProcessName -like "*server*" -or
        $_.ProcessName -like "*client*"
    } | Stop-Process -Force -ErrorAction SilentlyContinue
    
    Remove-Item -Path "*.log" -ErrorAction SilentlyContinue
}

try {
    Write-ColorOutput "`n=== xDS Basic Example Test ===" "Cyan"
    
    Write-ColorOutput "`nChecking prerequisites..." "Yellow"
    if (-not (Test-Command "go")) {
        Write-ColorOutput "Error: Go is not installed or not in PATH" "Red"
        exit 1
    }
    Write-ColorOutput "✓ Go found: $(go version)" "Green"
    
    Write-ColorOutput "`n=== Step 1: Starting xDS Control Plane ===" "Cyan"
    $controlPlanePath = Resolve-Path $ControlPlanePath
    $controlPlaneJob = Start-Job -ScriptBlock {
        param($Path)
        Set-Location $Path
        go run main.go --config config.yaml
    } -ArgumentList $controlPlanePath.FullName
    
    Start-Sleep -Seconds 2
    Write-ColorOutput "✓ Control plane started" "Green"
    
    Write-ColorOutput "`n=== Step 2: Starting Server ===" "Cyan"
    $serverPath = Resolve-Path $ServerPath
    $serverJob = Start-Job -ScriptBlock {
        param($Path)
        Set-Location $Path
        go run main.go --config config.yaml
    } -ArgumentList $serverPath.FullName
    
    Start-Sleep -Seconds 2
    Write-ColorOutput "✓ Server started" "Green"
    
    Write-ColorOutput "`n=== Step 3: Running Client ===" "Cyan"
    $clientPath = Resolve-Path $ClientPath
    $success = Start-ProcessWithTimeout -Path "go" -Arguments "run main.go --config config.yaml" -TimeoutSeconds $Timeout -Name "client"
    
    if (-not $success) {
        Write-ColorOutput "`n✗ Test failed: Client did not complete successfully" "Red"
        exit 1
    }
    
    Write-ColorOutput "`n✓ Client completed" "Green"
    
    Write-ColorOutput "`n=== Step 4: Verifying Output ===" "Cyan"
    $clientOutput = Get-Content "client.stdout.log"
    
    $successChecks = 0
    $totalChecks = 4
    
    if ($clientOutput -match "xDS basic client completed successfully") {
        Write-ColorOutput "✓ Client completed successfully" "Green"
        $successChecks++
    }
    
    if ($clientOutput -match "GetShelf response") {
        Write-ColorOutput "✓ GetShelf RPC called" "Green"
        $successChecks++
    }
    
    if ($clientOutput -match "CreateShelf response") {
        Write-ColorOutput "✓ CreateShelf RPC called" "Green"
        $successChecks++
    }
    
    if ($clientOutput -match "ListShelves response") {
        Write-ColorOutput "✓ ListShelves RPC called" "Green"
        $successChecks++
    }
    
    Write-ColorOutput "`n=== Test Results ===" "Cyan"
    Write-ColorOutput "Passed: $successChecks / $totalChecks checks" "Yellow"
    
    if ($successChecks -eq $totalChecks) {
        Write-ColorOutput "`n✓ All tests passed!" "Green"
        exit 0
    }
    else {
        Write-ColorOutput "`n✗ Some tests failed" "Red"
        Write-ColorOutput "Client output:" "Yellow"
        Get-Content "client.stdout.log" | Write-Host
        exit 1
    }
}
catch {
    Write-ColorOutput "`n✗ Test failed with exception: $_" "Red"
    Write-ColorOutput $_.ScriptStackTrace "Red"
    exit 1
}
finally {
    Cleanup
    
    if ($controlPlaneJob) {
        $controlPlaneJob | Stop-Job -ErrorAction SilentlyContinue
        $controlPlaneJob | Remove-Job -ErrorAction SilentlyContinue
    }
    
    if ($serverJob) {
        $serverJob | Stop-Job -ErrorAction SilentlyContinue
        $serverJob | Remove-Job -ErrorAction SilentlyContinue
    }
}

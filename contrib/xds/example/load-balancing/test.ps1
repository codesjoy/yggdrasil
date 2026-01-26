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
    xDS Load Balancing Test Script

.DESCRIPTION
    Automated test script for xDS load balancing example.
#>

$ErrorActionPreference = "Stop"

function Write-ColorOutput {
    param([string]$Message, [string]$Color = "Green")
    Write-Host $Message -ForegroundColor $Color
}

function Cleanup {
    Write-ColorOutput "`nCleaning up..." "Yellow"
    Get-Process | Where-Object {
        $_.ProcessName -like "*control-plane*" -or
        $_.ProcessName -like "*server*"
    } | Stop-Process -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "*.log" -ErrorAction SilentlyContinue
}

try {
    Write-ColorOutput "`n=== xDS Load Balancing Test ===" "Cyan"
    
    Write-ColorOutput "`n=== Step 1: Starting xDS Control Plane ===" "Cyan"
    $env:XDS_CONFIG_FILE = "load-balancing-xds-config.yaml"
    $controlPlanePath = Resolve-Path "../control-plane"
    $controlPlaneJob = Start-Job -ScriptBlock {
        param($Path)
        Set-Location $Path
        go run main.go --config config.yaml
    } -ArgumentList $controlPlanePath.FullName
    
    Start-Sleep -Seconds 2
    Write-ColorOutput "✓ Control plane started" "Green"
    
    Write-ColorOutput "`n=== Step 2: Starting Server Instance 1 ===" "Cyan"
    $env:SERVER_ID = "1"
    $serverPath = Resolve-Path "./server"
    $serverJob1 = Start-Job -ScriptBlock {
        param($Path)
        Set-Location $Path
        go run main.go --config config.yaml
    } -ArgumentList $serverPath.FullName
    
    Start-Sleep -Seconds 1
    Write-ColorOutput "✓ Server 1 started" "Green"
    
    Write-ColorOutput "`n=== Step 3: Starting Server Instance 2 ===" "Cyan"
    $env:SERVER_ID = "2"
    $env:PORT = "55556"
    $serverJob2 = Start-Job -ScriptBlock {
        param($Path)
        Set-Location $Path
        go run main.go --config config.yaml
    } -ArgumentList $serverPath.FullName
    
    Start-Sleep -Seconds 1
    Write-ColorOutput "✓ Server 2 started" "Green"
    
    Write-ColorOutput "`n=== Step 4: Starting Server Instance 3 ===" "Cyan"
    $env:SERVER_ID = "3"
    $env:PORT = "55557"
    $serverJob3 = Start-Job -ScriptBlock {
        param($Path)
        Set-Location $Path
        go run main.go --config config.yaml
    } -ArgumentList $serverPath.FullName
    
    Start-Sleep -Seconds 1
    Write-ColorOutput "✓ Server 3 started" "Green"
    
    Write-ColorOutput "`n=== Step 5: Running Client ===" "Cyan"
    $clientPath = Resolve-Path "./client"
    $process = Start-Process -FilePath "go" -ArgumentList "run main.go --config config.yaml" -NoNewWindow -PassThru -RedirectStandardOutput "client.stdout.log" -RedirectStandardError "client.stderr.log" -WorkingDirectory $clientPath.FullName
    
    $startTime = Get-Date
    while (-not $process.HasExited) {
        $elapsed = (Get-Date) - $startTime
        if ($elapsed.TotalSeconds -gt 30) {
            Write-ColorOutput "Timeout waiting for client" "Red"
            $process.Kill()
            break
        }
        Start-Sleep -Milliseconds 100
    }
    
    if ($process.ExitCode -ne 0) {
        Write-ColorOutput "✗ Client failed with exit code $($process.ExitCode)" "Red"
        Get-Content "client.stderr.log" | Write-Host
        exit 1
    }
    
    Write-ColorOutput "✓ Client completed" "Green"
    
    Write-ColorOutput "`n=== Step 6: Verifying Output ===" "Cyan"
    $clientOutput = Get-Content "client.stdout.log"
    
    $successChecks = 0
    $totalChecks = 3
    
    if ($clientOutput -match "Load balancing test completed") {
        Write-ColorOutput "✓ Test completed successfully" "Green"
        $successChecks++
    }
    
    if ($clientOutput -match "Traffic Distribution") {
        Write-ColorOutput "✓ Traffic distribution reported" "Green"
        $successChecks++
    }
    
    $server1Hits = ($clientOutput | Select-String "server_id: server-1" | Measure-Object).Count
    $server2Hits = ($clientOutput | Select-String "server_id: server-2" | Measure-Object).Count
    $server3Hits = ($clientOutput | Select-String "server_id: server-3" | Measure-Object).Count
    
    if ($server1Hits -gt 0 -and $server2Hits -gt 0 -and $server3Hits -gt 0) {
        Write-ColorOutput "✓ All servers received requests" "Green"
        Write-ColorOutput "  Server 1: $server1Hits requests" "Gray"
        Write-ColorOutput "  Server 2: $server2Hits requests" "Gray"
        Write-ColorOutput "  Server 3: $server3Hits requests" "Gray"
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
        exit 1
    }
}
catch {
    Write-ColorOutput "`n✗ Test failed: $_" "Red"
    exit 1
}
finally {
    Cleanup
    
    if ($controlPlaneJob) { $controlPlaneJob | Stop-Job -ErrorAction SilentlyContinue; $controlPlaneJob | Remove-Job -ErrorAction SilentlyContinue }
    if ($serverJob1) { $serverJob1 | Stop-Job -ErrorAction SilentlyContinue; $serverJob1 | Remove-Job -ErrorAction SilentlyContinue }
    if ($serverJob2) { $serverJob2 | Stop-Job -ErrorAction SilentlyContinue; $serverJob2 | Remove-Job -ErrorAction SilentlyContinue }
    if ($serverJob3) { $serverJob3 | Stop-Job -ErrorAction SilentlyContinue; $serverJob3 | Remove-Job -ErrorAction SilentlyContinue }
}

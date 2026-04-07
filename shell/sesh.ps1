# sesh - PowerShell wrapper for the sesh session picker
# Add this to your PowerShell profile ($PROFILE):
#   . /path/to/sesh.ps1
#
# Or copy the function directly into your profile.
#
# The wrapper is needed because the binary outputs a shell command
# (Set-Location + exec) that must run in the current session.

function sesh {
    $output = & sesh.exe @args
    if ($LASTEXITCODE -eq 0 -and $output) {
        Invoke-Expression $output
    }
}

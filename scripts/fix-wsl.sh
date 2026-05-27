#!/bin/bash
# =============================================================================
# WSL + VS Code/Antigravity IDE Connection Fix Script
# =============================================================================
# Run this when you get "Failed parsing install script output" errors
# or when WSL disconnects and won't reconnect.
#
# Usage: bash ~/goboxd/scripts/fix-wsl.sh
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   WSL + IDE Connection Fix Script                    ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════╝${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 1: Kill stale VS Code server processes
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[1/5] Killing stale VS Code server processes...${NC}"
pkill -f "vscode-server" 2>/dev/null && echo "  → Killed vscode-server processes" || echo "  → No stale vscode-server processes found"
pkill -f "code-server" 2>/dev/null && echo "  → Killed code-server processes" || echo "  → No stale code-server processes found"
# Give processes time to die
sleep 1
echo -e "${GREEN}  ✓ Done${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 2: Clean corrupted server binary (will auto-reinstall on next connection)
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[2/5] Cleaning corrupted server installation...${NC}"
if [ -d "$HOME/.vscode-server/bin" ]; then
    rm -rf "$HOME/.vscode-server/bin"
    echo "  → Removed server binaries (will be reinstalled automatically)"
else
    echo "  → No server binaries to clean"
fi

# Clean stale lock files
rm -f /tmp/.vscode-server-* 2>/dev/null && echo "  → Cleaned lock files" || true
rm -f /tmp/vscode-*.sock 2>/dev/null && echo "  → Cleaned socket files" || true
echo -e "${GREEN}  ✓ Done${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 3: Clean old server logs (preserves settings & extensions)
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[3/5] Cleaning old log files...${NC}"
if [ -d "$HOME/.vscode-server/data/logs" ]; then
    rm -rf "$HOME/.vscode-server/data/logs"
    echo "  → Removed old log files"
else
    echo "  → No old logs to clean"
fi

# Clean IPC files
rm -rf /tmp/vscode-ipc-* 2>/dev/null && echo "  → Cleaned IPC files" || true
echo -e "${GREEN}  ✓ Done${NC}"
echo ""

# -----------------------------------------------------------------------------
# Step 4: Clean corrupted systemd journals (if sudo available)
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[4/5] Checking systemd journal health...${NC}"
CORRUPTED_COUNT=$(find /var/log/journal -name "*.journal~" 2>/dev/null | wc -l)
if [ "$CORRUPTED_COUNT" -gt 0 ]; then
    echo -e "  ${RED}→ Found $CORRUPTED_COUNT corrupted journal files${NC}"
    echo "  → Attempting cleanup (may require sudo password)..."
    if sudo -n true 2>/dev/null; then
        sudo find /var/log/journal -name "*.journal~" -delete 2>/dev/null
        sudo journalctl --vacuum-size=100M 2>/dev/null
        echo -e "${GREEN}  ✓ Journals cleaned${NC}"
    else
        echo -e "  ${YELLOW}→ Skipped (no passwordless sudo). Run manually:${NC}"
        echo "    sudo find /var/log/journal -name '*.journal~' -delete"
        echo "    sudo journalctl --vacuum-size=100M"
    fi
else
    echo "  → Journals are clean"
fi
echo ""

# -----------------------------------------------------------------------------
# Step 5: Verify system health
# -----------------------------------------------------------------------------
echo -e "${YELLOW}[5/5] Verifying system health...${NC}"

# Check memory
MEM_AVAIL=$(awk '/MemAvailable/ {print int($2/1024)}' /proc/meminfo 2>/dev/null)
if [ -n "$MEM_AVAIL" ] && [ "$MEM_AVAIL" -lt 512 ]; then
    echo -e "  ${RED}⚠ Low memory: ${MEM_AVAIL}MB available. Consider closing other apps.${NC}"
else
    echo -e "  ${GREEN}✓ Memory OK: ${MEM_AVAIL}MB available${NC}"
fi

# Check disk
DISK_USE=$(df / 2>/dev/null | awk 'NR==2 {print $5}' | tr -d '%')
if [ -n "$DISK_USE" ] && [ "$DISK_USE" -gt 90 ]; then
    echo -e "  ${RED}⚠ Disk usage high: ${DISK_USE}%${NC}"
else
    echo -e "  ${GREEN}✓ Disk OK: ${DISK_USE}% used${NC}"
fi

# Check DNS
if ping -c 1 -W 2 8.8.8.8 &>/dev/null; then
    echo -e "  ${GREEN}✓ Network connectivity OK${NC}"
else
    echo -e "  ${RED}⚠ Network connectivity issue detected${NC}"
fi

echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Fix Complete!                                      ║${NC}"
echo -e "${BLUE}╠══════════════════════════════════════════════════════╣${NC}"
echo -e "${BLUE}║                                                      ║${NC}"
echo -e "${BLUE}║   Next steps:                                        ║${NC}"
echo -e "${BLUE}║   1. Close the IDE window completely                 ║${NC}"
echo -e "${BLUE}║   2. In PowerShell (Windows), run:                   ║${NC}"
echo -e "${BLUE}║      wsl --shutdown                                  ║${NC}"
echo -e "${BLUE}║   3. Wait 5 seconds, then reopen the IDE             ║${NC}"
echo -e "${BLUE}║                                                      ║${NC}"
echo -e "${BLUE}║   If still failing, also run in PowerShell:          ║${NC}"
echo -e "${BLUE}║      wsl --update                                    ║${NC}"
echo -e "${BLUE}║                                                      ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════╝${NC}"

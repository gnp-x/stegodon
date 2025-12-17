# VHS tape file for stegodon demo
# This creates a demo GIF showing registration and posting with Markdown links

Output demo.gif

Set Shell zsh

Set FontSize 14
Set Width 1300
Set Height 900

# Note: Stegodon must be running with fresh database
# Start with: rm database.db && ./stegodon

# Wait a moment
Sleep 1s

# Connect via SSH
Type "ssh localhost -p 23232"
Enter
Sleep 2s

# First-time registration - enter username
Type "stegodon"
Sleep 1s
Enter

# Enter display name
Type "Stegodon"
Sleep 1s
Enter

# Enter bio
Type "Testing stegodon's Markdown links feature"
Sleep 1s
Enter

# We're now in the TUI - should be at "New Note" view by default
# Type a note with a Markdown link
Type "Welcome to [Stegodon](https://github.com/deemkeen/stegodon)!"
Sleep 500ms
Enter
Enter
Type "A federated #microblog accessible via SSH."
Sleep 500ms
Enter
Enter

# Save the note with Ctrl+S
Ctrl+S
Sleep 1s

# Navigate to Notes List with Tab
Tab

# Show the posted note with clickable link (in green)
Sleep 1s

# Navigate back to New Note with Shift+Tab
Shift+Tab
Sleep 1s

# Create another note
Type "Markdown links work everywhere:"
Sleep 500ms
Enter
Type "TUI (green, clickable), "
Sleep 500ms
Type "Web UI (blue links), "
Sleep 500ms
Type "Federation (HTML)"
Sleep 1s

# Save second note
Ctrl+S
Sleep 1s

# Navigate to Notes List again
Tab
Sleep 1s

# Quit
Type "q"
Sleep 1s

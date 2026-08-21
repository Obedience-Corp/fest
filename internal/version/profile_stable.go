//go:build !dev

package version

// Profile is "stable" when built without the dev build tag.
const Profile = "stable"

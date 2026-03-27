import { createFileRoute } from "@tanstack/react-router";
import { Check, Copy, Key, Lock, Monitor, Moon, Save, ShieldCheck, Sun, User } from "lucide-react";
import { QRCodeSVG } from "qrcode.react";
import { useEffect, useState } from "react";
import { LoadingScreen } from "@/components/loading-screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import {
  useAvatars,
  useChangePassword,
  useCurrentUser,
  useDisable2FA,
  useSetup2FA,
  useUpdateProfile,
  useVerify2FA,
} from "@/hooks/use-auth";
import { getTheme, setTheme, type Theme } from "@/lib/theme";
import { cn } from "@/lib/utils";

export const Route = createFileRoute("/_dashboard/profile")({
  component: ProfilePage,
});

const AVATAR_EMOJI: Record<string, string> = {
  bear: "\u{1F43B}",
  cat: "\u{1F431}",
  dog: "\u{1F436}",
  fox: "\u{1F98A}",
  koala: "\u{1F428}",
  lion: "\u{1F981}",
  monkey: "\u{1F435}",
  owl: "\u{1F989}",
  panda: "\u{1F43C}",
  penguin: "\u{1F427}",
  rabbit: "\u{1F430}",
  tiger: "\u{1F42F}",
  whale: "\u{1F433}",
  wolf: "\u{1F43A}",
};

function ProfilePage() {
  const { data: user, isLoading } = useCurrentUser();
  const { data: avatars } = useAvatars();
  const updateProfile = useUpdateProfile();
  const changePassword = useChangePassword();
  const setup2FA = useSetup2FA();
  const verify2FA = useVerify2FA();
  const disable2FA = useDisable2FA();

  // Profile form state
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [selectedAvatar, setSelectedAvatar] = useState<string | undefined>(undefined);

  // Password form state
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordError, setPasswordError] = useState("");

  // 2FA state
  const [tfaSetupData, setTfaSetupData] = useState<{ secret: string; qr_code: string } | null>(
    null,
  );
  const [tfaCode, setTfaCode] = useState("");
  const [disableCode, setDisableCode] = useState("");
  const [showDisable2FA, setShowDisable2FA] = useState(false);
  const [secretCopied, setSecretCopied] = useState(false);

  useEffect(() => {
    if (user) {
      setFirstName(user.first_name || "");
      setLastName(user.last_name || "");
      setDisplayName(user.display_name || "");
      setSelectedAvatar(user.avatar_url);
    }
  }, [user]);

  if (isLoading) return <LoadingScreen />;
  if (!user) return null;

  const passwordsMatch = newPassword === confirmPassword;
  const passwordLongEnough = newPassword.length >= 8;
  const canChangePassword =
    currentPassword.length > 0 &&
    newPassword.length > 0 &&
    confirmPassword.length > 0 &&
    passwordsMatch &&
    passwordLongEnough;

  const handleSaveProfile = () => {
    updateProfile.mutate({
      first_name: firstName,
      last_name: lastName,
      display_name: displayName,
      avatar_url: selectedAvatar,
    });
  };

  const handleChangePassword = () => {
    if (!passwordsMatch) {
      setPasswordError("Passwords do not match");
      return;
    }
    if (!passwordLongEnough) {
      setPasswordError("Password must be at least 8 characters");
      return;
    }
    setPasswordError("");
    changePassword.mutate(
      { current_password: currentPassword, new_password: newPassword },
      {
        onSuccess: () => {
          setCurrentPassword("");
          setNewPassword("");
          setConfirmPassword("");
        },
      },
    );
  };

  const handleEnable2FA = () => {
    setup2FA.mutate(undefined, {
      onSuccess: (data) => {
        setTfaSetupData(data);
        setTfaCode("");
      },
    });
  };

  const handleVerify2FA = () => {
    verify2FA.mutate(tfaCode, {
      onSuccess: () => {
        setTfaSetupData(null);
        setTfaCode("");
      },
    });
  };

  const handleDisable2FA = () => {
    disable2FA.mutate(disableCode, {
      onSuccess: () => {
        setDisableCode("");
      },
    });
  };

  const copySecret = () => {
    if (tfaSetupData?.secret) {
      navigator.clipboard.writeText(tfaSetupData.secret);
      setSecretCopied(true);
      setTimeout(() => setSecretCopied(false), 2000);
    }
  };

  return (
    <div>
      <h1 className="text-2xl font-bold tracking-tight">Profile</h1>
      <p className="text-sm text-muted-foreground">Manage your account settings</p>

      <Separator className="my-6" />

      <div className="space-y-6">
        {/* Personal Info */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <User className="h-4 w-4" /> Personal Info
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            {/* Avatar grid */}
            {avatars && avatars.length > 0 && (
              <div className="space-y-2">
                <Label>Avatar</Label>
                <div className="flex flex-wrap gap-3">
                  {avatars.map((key) => (
                    <button
                      key={key}
                      type="button"
                      onClick={() => setSelectedAvatar(key)}
                      className={cn(
                        "flex h-12 w-12 items-center justify-center rounded-full text-2xl transition-all hover:scale-110",
                        selectedAvatar === key
                          ? "ring-2 ring-primary ring-offset-2 ring-offset-background"
                          : "ring-1 ring-border hover:ring-muted-foreground",
                      )}
                      title={key}
                    >
                      {AVATAR_EMOJI[key] || key[0]?.toUpperCase()}
                    </button>
                  ))}
                </div>
              </div>
            )}

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label>First Name</Label>
                <Input
                  value={firstName}
                  onChange={(e) => setFirstName(e.target.value)}
                  placeholder="First name"
                />
              </div>
              <div className="space-y-2">
                <Label>Last Name</Label>
                <Input
                  value={lastName}
                  onChange={(e) => setLastName(e.target.value)}
                  placeholder="Last name"
                />
              </div>
            </div>

            <div className="space-y-2">
              <Label>Display Name</Label>
              <Input
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Display name"
                className="max-w-md"
              />
            </div>

            <div className="space-y-2">
              <Label>Email</Label>
              <Input value={user.email} disabled className="max-w-md font-mono opacity-60" />
            </div>

            <div className="flex justify-end">
              <Button onClick={handleSaveProfile} disabled={updateProfile.isPending}>
                <Save className="h-3.5 w-3.5" />
                {updateProfile.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Change Password */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <Lock className="h-4 w-4" /> Change Password
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <Label>Current Password</Label>
              <Input
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                placeholder="Current password"
                className="max-w-md"
              />
            </div>
            <div className="space-y-2">
              <Label>New Password</Label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => {
                  setNewPassword(e.target.value);
                  setPasswordError("");
                }}
                placeholder="New password (min. 8 characters)"
                className="max-w-md"
              />
            </div>
            <div className="space-y-2">
              <Label>Confirm Password</Label>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(e) => {
                  setConfirmPassword(e.target.value);
                  setPasswordError("");
                }}
                placeholder="Confirm new password"
                className="max-w-md"
              />
              {confirmPassword.length > 0 && !passwordsMatch && (
                <p className="text-xs text-destructive">Passwords do not match</p>
              )}
              {newPassword.length > 0 && !passwordLongEnough && (
                <p className="text-xs text-destructive">Password must be at least 8 characters</p>
              )}
              {passwordError && <p className="text-xs text-destructive">{passwordError}</p>}
            </div>
            <div className="flex justify-end">
              <Button
                onClick={handleChangePassword}
                disabled={!canChangePassword || changePassword.isPending}
              >
                <Key className="h-3.5 w-3.5" />
                {changePassword.isPending ? "Changing..." : "Change Password"}
              </Button>
            </div>
          </CardContent>
        </Card>

        {/* Two-Factor Authentication */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm font-medium">
              <ShieldCheck className="h-4 w-4" /> Two-Factor Authentication
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {!user.two_fa_enabled && !tfaSetupData && (
              <div className="space-y-3">
                <p className="text-sm text-muted-foreground">
                  Protect your account with two-factor authentication. You will need an
                  authenticator app like Google Authenticator or Authy.
                </p>
                <Button onClick={handleEnable2FA} disabled={setup2FA.isPending}>
                  <ShieldCheck className="h-3.5 w-3.5" />
                  {setup2FA.isPending ? "Setting up..." : "Enable 2FA"}
                </Button>
              </div>
            )}

            {!user.two_fa_enabled && tfaSetupData && (
              <div className="space-y-4">
                <div className="grid gap-6 sm:grid-cols-[2fr_1fr]">
                  {/* Left: instructions + secret + verify */}
                  <div className="space-y-4">
                    <p className="text-sm text-muted-foreground">
                      Scan the QR code with your authenticator app, or enter the secret manually:
                    </p>
                    <div className="flex items-center gap-2">
                      <code className="flex-1 rounded border bg-muted px-3 py-2 font-mono text-xs break-all">
                        {tfaSetupData.secret}
                      </code>
                      <Button variant="outline" size="sm" onClick={copySecret}>
                        {secretCopied ? (
                          <Check className="h-3.5 w-3.5" />
                        ) : (
                          <Copy className="h-3.5 w-3.5" />
                        )}
                      </Button>
                    </div>
                    <div className="space-y-2">
                      <Label>Verification Code</Label>
                      <div className="flex items-center gap-3">
                        <Input
                          value={tfaCode}
                          onChange={(e) =>
                            setTfaCode(e.target.value.replace(/\D/g, "").slice(0, 6))
                          }
                          placeholder="6-digit code"
                          className="max-w-[180px] font-mono"
                          maxLength={6}
                        />
                        <Button
                          onClick={handleVerify2FA}
                          disabled={tfaCode.length !== 6 || verify2FA.isPending}
                        >
                          {verify2FA.isPending ? "Verifying..." : "Verify & Enable"}
                        </Button>
                      </div>
                    </div>
                  </div>

                  {/* Right: QR code */}
                  <div className="flex items-start justify-center">
                    <div className="rounded-xl border bg-white p-4 shadow-sm">
                      <QRCodeSVG value={tfaSetupData.qr_code} size={160} />
                    </div>
                  </div>
                </div>
              </div>
            )}

            {user.two_fa_enabled && !showDisable2FA && (
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-emerald-500/10">
                    <Check className="h-4 w-4 text-emerald-500" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">Two-factor authentication is enabled</p>
                    <p className="text-xs text-muted-foreground">
                      Your account is protected with 2FA
                    </p>
                  </div>
                </div>
                <Button variant="outline" size="sm" onClick={() => setShowDisable2FA(true)}>
                  Disable
                </Button>
              </div>
            )}

            {user.two_fa_enabled && showDisable2FA && (
              <div className="space-y-4">
                <div className="rounded-md border border-destructive/30 bg-destructive/5 p-4">
                  <p className="text-sm font-medium text-destructive">
                    Disable two-factor authentication
                  </p>
                  <p className="mt-1 text-xs text-destructive/80">
                    Enter the 6-digit code from your authenticator app to confirm.
                  </p>
                </div>
                <div className="flex items-center gap-3">
                  <Input
                    value={disableCode}
                    onChange={(e) => setDisableCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                    placeholder="000000"
                    className="max-w-[180px] text-center font-mono text-lg tracking-widest"
                    maxLength={6}
                    autoFocus
                  />
                  <Button
                    variant="destructive"
                    onClick={handleDisable2FA}
                    disabled={disableCode.length !== 6 || disable2FA.isPending}
                  >
                    {disable2FA.isPending ? "Disabling..." : "Confirm"}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => {
                      setShowDisable2FA(false);
                      setDisableCode("");
                    }}
                  >
                    Cancel
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Appearance */}
        <AppearanceCard />
      </div>
    </div>
  );
}

// ── Appearance Card ─────────────────────────────────────────────────

function AppearanceCard() {
  const [currentTheme, setCurrentTheme] = useState<Theme>(getTheme);

  const options = [
    { value: "dark" as const, icon: Moon, label: "Dark" },
    { value: "light" as const, icon: Sun, label: "Light" },
    { value: "system" as const, icon: Monitor, label: "System" },
  ];

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <Sun className="h-4 w-4" /> Appearance
        </CardTitle>
      </CardHeader>
      <CardContent>
        <Label className="mb-3 block text-sm">Theme</Label>
        <div className="inline-flex rounded-lg border bg-muted p-1">
          {options.map((option) => {
            const Icon = option.icon;
            const active = currentTheme === option.value;
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => {
                  setCurrentTheme(option.value);
                  setTheme(option.value);
                }}
                className={cn(
                  "flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium transition-all",
                  active
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                <Icon className="h-3.5 w-3.5" />
                {option.label}
              </button>
            );
          })}
        </div>
      </CardContent>
    </Card>
  );
}

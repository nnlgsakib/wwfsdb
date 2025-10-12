"use client"

import { authFetch } from "@/lib/authFetch";
import { useEffect, useState } from "react"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { User, Shield, Bell, LogOut, Edit, Save, Smartphone, Monitor } from "lucide-react"
import { useRouter } from "next/navigation"

interface Profile {
  user_id: string
  username: string
  email: string
  full_name: string
  phone_number: string
  address: string
  created_at: string
  updated_at: string
}

interface SecurityLog {
  log_id: string
  action: string
  ip_address: string
  device_info: string
  timestamp: string
}

interface Setting {
  setting_id: string
  setting_name: string
  setting_value: string
}

export default function ProfilePage() {
  const router = useRouter()
  const [profile, setProfile] = useState<Profile | null>(null)
  const [securityLogs, setSecurityLogs] = useState<SecurityLog[]>([])
  const [settings, setSettings] = useState<Setting[]>([])
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState(false)
  const [formData, setFormData] = useState({
    full_name: "",
    email: "",
    phone_number: "",
    address: "",
  })

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [profileRes, logsRes, settingsRes] = await Promise.all([
          authFetch("/api/profile"),
          authFetch("/api/security-logs"),
          authFetch("/api/settings"),
        ])

        const profileData = await profileRes.json()
        const logsData = await logsRes.json()
        const settingsData = await settingsRes.json()

        if (profileData.success) {
          setProfile(profileData.profile)
          setFormData({
            full_name: profileData.profile.full_name,
            email: profileData.profile.email,
            phone_number: profileData.profile.phone_number,
            address: profileData.profile.address,
          })
        }
        if (logsData.success) setSecurityLogs(logsData.logs)
        if (settingsData.success) setSettings(settingsData.settings)
      } catch (error) {
        console.error("[v0] Error fetching data:", error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  const handleSave = async () => {
    try {
      const response = await authFetch("/api/profile", {
        method: "PUT",
        body: JSON.stringify(formData),
      })

      const data = await response.json()

      if (data.success) {
        setProfile(data.profile)
        setEditing(false)
      }
    } catch (error) {
      console.error("[v0] Error updating profile:", error)
    }
  }

  const handleSettingChange = async (setting_name: string, setting_value: string) => {
    try {
      await authFetch("/api/settings", {
        method: "PUT",
        body: JSON.stringify({ setting_name, setting_value }),
      })

      setSettings(settings.map((s) => (s.setting_name === setting_name ? { ...s, setting_value } : s)))
    } catch (error) {
      console.error("[v0] Error updating setting:", error)
    }
  }

  const handleLogout = () => {
    localStorage.removeItem("token")
    localStorage.removeItem("user")
    router.push("/login")
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    )
  }

  if (!profile) return null

  const getSetting = (name: string) => settings.find((s) => s.setting_name === name)?.setting_value === "true"

  return (
    <div className="p-4 md:p-8 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-balance">Profile & Settings</h1>
          <p className="text-muted-foreground mt-1">Manage your account and preferences</p>
        </div>
        <Button variant="destructive" onClick={handleLogout} className="gap-2">
          <LogOut className="w-4 h-4" />
          <span className="hidden sm:inline">Logout</span>
        </Button>
      </div>

      <Tabs defaultValue="profile" className="space-y-6">
        <TabsList className="grid w-full grid-cols-3 lg:w-auto">
          <TabsTrigger value="profile" className="gap-2">
            <User className="w-4 h-4" />
            <span className="hidden sm:inline">Profile</span>
          </TabsTrigger>
          <TabsTrigger value="security" className="gap-2">
            <Shield className="w-4 h-4" />
            <span className="hidden sm:inline">Security</span>
          </TabsTrigger>
          <TabsTrigger value="settings" className="gap-2">
            <Bell className="w-4 h-4" />
            <span className="hidden sm:inline">Settings</span>
          </TabsTrigger>
        </TabsList>

        {/* Profile Tab */}
        <TabsContent value="profile" className="space-y-6">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between">
              <CardTitle>Personal Information</CardTitle>
              {!editing ? (
                <Button variant="outline" size="sm" onClick={() => setEditing(true)} className="gap-2 bg-transparent">
                  <Edit className="w-4 h-4" />
                  Edit
                </Button>
              ) : (
                <div className="flex gap-2">
                  <Button variant="outline" size="sm" onClick={() => setEditing(false)} className="bg-transparent">
                    Cancel
                  </Button>
                  <Button size="sm" onClick={handleSave} className="gap-2">
                    <Save className="w-4 h-4" />
                    Save
                  </Button>
                </div>
              )}
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-2">
                  <Label htmlFor="full_name">Full Name</Label>
                  <Input
                    id="full_name"
                    value={formData.full_name}
                    onChange={(e) => setFormData({ ...formData, full_name: e.target.value })}
                    disabled={!editing}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="username">Username</Label>
                  <Input id="username" value={profile.username} disabled />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    value={formData.email}
                    onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                    disabled={!editing}
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="phone">Phone Number</Label>
                  <Input
                    id="phone"
                    value={formData.phone_number}
                    onChange={(e) => setFormData({ ...formData, phone_number: e.target.value })}
                    disabled={!editing}
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="address">Address</Label>
                <Textarea
                  id="address"
                  value={formData.address}
                  onChange={(e) => setFormData({ ...formData, address: e.target.value })}
                  disabled={!editing}
                  rows={3}
                />
              </div>
              <div className="pt-4 border-t">
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Account Created</span>
                  <span className="font-medium">
                    {new Date(profile.created_at).toLocaleDateString("en-US", {
                      month: "long",
                      day: "numeric",
                      year: "numeric",
                    })}
                  </span>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Security Tab */}
        <TabsContent value="security" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Security Activity</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {securityLogs.map((log) => (
                  <div key={log.log_id} className="flex items-start gap-4 p-4 rounded-lg border">
                    <div className="w-10 h-10 bg-primary/10 rounded-full flex items-center justify-center flex-shrink-0">
                      {log.device_info.includes("iPhone") || log.device_info.includes("Safari") ? (
                        <Smartphone className="w-5 h-5 text-primary" />
                      ) : (
                        <Monitor className="w-5 h-5 text-primary" />
                      )}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-start justify-between gap-4 mb-1">
                        <p className="font-semibold">{log.action}</p>
                        <span className="text-xs text-muted-foreground whitespace-nowrap">
                          {new Date(log.timestamp).toLocaleDateString("en-US", {
                            month: "short",
                            day: "numeric",
                            hour: "2-digit",
                            minute: "2-digit",
                          })}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground">{log.device_info}</p>
                      <p className="text-xs text-muted-foreground mt-1">IP: {log.ip_address}</p>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Change Password</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="current_password">Current Password</Label>
                <Input id="current_password" type="password" placeholder="Enter current password" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="new_password">New Password</Label>
                <Input id="new_password" type="password" placeholder="Enter new password" />
              </div>
              <div className="space-y-2">
                <Label htmlFor="confirm_password">Confirm New Password</Label>
                <Input id="confirm_password" type="password" placeholder="Confirm new password" />
              </div>
              <Button className="w-full">Update Password</Button>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Settings Tab */}
        <TabsContent value="settings" className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Notifications</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="notifications">Push Notifications</Label>
                  <p className="text-sm text-muted-foreground">Receive notifications about your account activity</p>
                </div>
                <Switch
                  id="notifications"
                  checked={getSetting("notifications_enabled")}
                  onCheckedChange={(checked) => handleSettingChange("notifications_enabled", checked.toString())}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="email_alerts">Email Alerts</Label>
                  <p className="text-sm text-muted-foreground">Get email notifications for transactions</p>
                </div>
                <Switch
                  id="email_alerts"
                  checked={getSetting("email_alerts")}
                  onCheckedChange={(checked) => handleSettingChange("email_alerts", checked.toString())}
                />
              </div>
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="sms_alerts">SMS Alerts</Label>
                  <p className="text-sm text-muted-foreground">Receive text messages for important updates</p>
                </div>
                <Switch
                  id="sms_alerts"
                  checked={getSetting("sms_alerts")}
                  onCheckedChange={(checked) => handleSettingChange("sms_alerts", checked.toString())}
                />
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Security Settings</CardTitle>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <Label htmlFor="two_factor">Two-Factor Authentication</Label>
                  <p className="text-sm text-muted-foreground">Add an extra layer of security to your account</p>
                </div>
                <Switch
                  id="two_factor"
                  checked={getSetting("two_factor_auth")}
                  onCheckedChange={(checked) => handleSettingChange("two_factor_auth", checked.toString())}
                />
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

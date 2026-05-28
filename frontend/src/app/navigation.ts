import {
  Home,
  Inbox,
  MessageSquareText,
  RadioTower,
  Repeat2,
  Send,
  Settings,
  Users,
} from "lucide-react";

export type NavItem = {
  href: string;
  label: string;
  description: string;
  icon: typeof Home;
};

export const navItems: NavItem[] = [
  {
    href: "#/",
    label: "Overview",
    description: "Instance health and recent activity",
    icon: Home,
  },
  {
    href: "#/posts",
    label: "Posts",
    description: "Write and manage local notes",
    icon: MessageSquareText,
  },
  {
    href: "#/follows",
    label: "Follows",
    description: "People following and followed by this instance",
    icon: Users,
  },
  {
    href: "#/inbox",
    label: "Inbox",
    description: "Inbound federation activity",
    icon: Inbox,
  },
  {
    href: "#/outbox",
    label: "Outbox",
    description: "Published activities and fanout",
    icon: Send,
  },
  {
    href: "#/delivery",
    label: "Delivery",
    description: "Remote delivery attempts and retries",
    icon: Repeat2,
  },
  {
    href: "#/compatibility",
    label: "Compatibility",
    description: "Connection and federation health",
    icon: RadioTower,
  },
  {
    href: "#/settings",
    label: "Settings",
    description: "Instance and account preferences",
    icon: Settings,
  },
];

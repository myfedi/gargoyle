import {
  Inbox,
  MessageSquareText,
  Repeat2,
  Send,
  Settings,
  Users,
} from "lucide-react";

export type NavItem = {
  href: string;
  label: string;
  description: string;
  icon: typeof MessageSquareText;
};

export const navItems: NavItem[] = [
  {
    href: "#/",
    label: "Timeline",
    description: "Posts and recent activity",
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
    href: "#/settings",
    label: "Settings",
    description: "Instance and account preferences",
    icon: Settings,
  },
];

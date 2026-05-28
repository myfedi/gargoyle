import {
  Bookmark,
  Heart,
  Mail,
  MessageSquareText,
  Repeat2,
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
    href: "#/bookmarks",
    label: "Bookmarks",
    description: "Saved posts",
    icon: Bookmark,
  },
  {
    href: "#/favourites",
    label: "Favourites",
    description: "Favourited posts",
    icon: Heart,
  },
  {
    href: "#/direct",
    label: "Direct",
    description: "Private conversations",
    icon: Mail,
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

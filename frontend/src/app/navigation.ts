import {
  Mail,
  MessageSquareText,
  Search,
  UserCircle,
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
    href: "#/search",
    label: "Search",
    description: "Find people",
    icon: Search,
  },
  {
    href: "#/profile",
    label: "My profile",
    description: "Profile, follows, bookmarks, and favourites",
    icon: UserCircle,
  },
  {
    href: "#/direct",
    label: "DMs",
    description: "Private conversations",
    icon: Mail,
  },
];

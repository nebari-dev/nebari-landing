import type{
  ReactNode
} from "react";

import {
  Card, CardBody, Tag
} from "@trussworks/react-uswds";

import
  AvatarImage
from "../icon";

import {
  cn
} from "../../lib/cn";

import "./index.scss";

export function SimpleCard(props: SimpleCardProps): ReactNode {
  
  // Extract the props
  const { className, image, name, status } = props;

  return (
    <Card className={cn("app-card app-card--simple", className)} layout="standardDefault">
      <CardBody className="app-card__body app-card__body--simple">
        <div className="app-card-simple__row">
          <div className="app-card-simple__avatar">
            {AvatarImage(image)}
          </div>

          <div className="app-card-simple__meta">
            <h3 className="app-card__title app-card__title--simple">{name}</h3>
            <Tag background="base-lighter" className="app-card__status app-card__status--simple">
              {status}
            </Tag>
          </div>
        </div>
      </CardBody>
    </Card>
  );
}

export type SimpleCardProps = {
  className?: string;
  image: string;
  name: string;
  status: string;
};
